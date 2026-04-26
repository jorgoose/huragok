package create

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jorgoose/huragok/internal/cliresult"
	"github.com/jorgoose/huragok/internal/display"
	"github.com/jorgoose/huragok/internal/provider"
	"github.com/jorgoose/huragok/internal/runs"
)

type Options struct {
	Prompt      string
	OutputPath  string
	JSON        bool
	From        string  // path to a user-supplied image; bypasses OpenAI
	WorkDir     string  // root for run directories; defaults to ".huragok" when empty
	MaxCostUSD  float64 // 0 = no cap; otherwise abort before the Hunyuan call if cumulative would exceed
}

// Run executes the create pipeline. Returns a Result describing what happened
// (always non-nil) and a *cliresult.PipelineError on failure whose Code maps
// to the process exit code.
func Run(ctx context.Context, opts Options) (*cliresult.Result, error) {
	started := time.Now()
	result := &cliresult.Result{Stages: map[string]cliresult.StageBrief{}}

	finalize := func(err error) (*cliresult.Result, error) {
		result.ElapsedSeconds = time.Since(started).Seconds()
		if err != nil {
			result.Status = cliresult.StatusFailed
			pe, ok := err.(*cliresult.PipelineError)
			if !ok {
				pe = &cliresult.PipelineError{Code: cliresult.ExitStage, Stage: "unknown", Err: err}
			}
			result.Error = &cliresult.ResultError{Stage: pe.Stage, Message: pe.Err.Error()}
			if !opts.JSON {
				display.Error(pe.Err.Error())
			}
			return result, pe
		}
		result.Status = cliresult.StatusComplete
		return result, nil
	}

	// OpenAI is only required when generating a concept image — --from skips it.
	openaiKey := os.Getenv("HURAGOK_OPENAI_KEY")
	if opts.From == "" && openaiKey == "" {
		return finalize(cliresult.ConfigError(fmt.Errorf("HURAGOK_OPENAI_KEY environment variable is required")))
	}
	hunyuanSecretID := os.Getenv("HURAGOK_HUNYUAN_SECRET_ID")
	if hunyuanSecretID == "" {
		return finalize(cliresult.ConfigError(fmt.Errorf("HURAGOK_HUNYUAN_SECRET_ID environment variable is required")))
	}
	hunyuanSecretKey := os.Getenv("HURAGOK_HUNYUAN_SECRET_KEY")
	if hunyuanSecretKey == "" {
		return finalize(cliresult.ConfigError(fmt.Errorf("HURAGOK_HUNYUAN_SECRET_KEY environment variable is required")))
	}

	outDir := filepath.Dir(opts.OutputPath)
	if outDir != "" && outDir != "." {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return finalize(cliresult.ConfigError(fmt.Errorf("creating output directory: %w", err)))
		}
	}

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = ".huragok"
	}
	run, err := runs.New(workDir, opts.Prompt, opts.OutputPath)
	if err != nil {
		return finalize(cliresult.ConfigError(err))
	}
	result.RunID = run.Meta.RunID

	if !opts.JSON {
		display.Header()
		if opts.Prompt != "" {
			display.Prompt(opts.Prompt)
		}
		display.RunID(run.Meta.RunID)
	}

	// Stage 1: concept image — either generate via OpenAI or use --from.
	var imgResult *provider.ImageResult
	if opts.From != "" {
		_ = run.SetImageSource("user")
		dst := filepath.Join(run.Dir(), "concept.png")
		src, err := os.ReadFile(opts.From)
		if err != nil {
			return finalize(cliresult.ConfigError(fmt.Errorf("reading --from image %s: %w", opts.From, err)))
		}
		if err := os.WriteFile(dst, src, 0o644); err != nil {
			return finalize(cliresult.StageError("io", fmt.Errorf("copying user image to run dir: %w", err)))
		}
		imgResult = &provider.ImageResult{Path: dst}
		_ = run.MarkStage("image", runs.StatusComplete, 0, nil)
		// --from costs nothing in stage 1.
		result.Stages["image"] = cliresult.StageBrief{Status: cliresult.StatusComplete, ElapsedMs: 0}
		if !opts.JSON {
			display.StageInfo(fmt.Sprintf("Using image → %s", dst))
			fmt.Println()
		}
	} else {
		_ = run.SetImageSource("openai")
		imgStart := time.Now()
		if !opts.JSON {
			display.StageStart("Generating concept image...")
		}
		var err error
		imgResult, err = provider.GenerateImage(ctx, openaiKey, opts.Prompt, run.Dir())
		imgElapsed := time.Since(imgStart)
		if err != nil {
			_ = run.MarkStage("image", runs.StatusFailed, imgElapsed, err)
			_ = run.SetStatus(runs.StatusFailed)
			result.Stages["image"] = cliresult.StageBrief{Status: cliresult.StatusFailed, ElapsedMs: imgElapsed.Milliseconds()}
			return finalize(cliresult.StageError("image", err))
		}
		_ = run.MarkStage("image", runs.StatusComplete, imgElapsed, nil)
		_ = run.SetStageCost("image", provider.DallE3StandardCostUSD)
		result.Stages["image"] = cliresult.StageBrief{
			Status:          cliresult.StatusComplete,
			ElapsedMs:       imgElapsed.Milliseconds(),
			CostEstimateUSD: provider.DallE3StandardCostUSD,
		}
		result.CostEstimateUSD += provider.DallE3StandardCostUSD
		if !opts.JSON {
			display.StageDone(imgStart)
			display.StageInfo(fmt.Sprintf("Saved → %s", imgResult.Path))
			fmt.Println()
		}
	}

	// Pre-flight max-cost check before the expensive Hunyuan call.
	if opts.MaxCostUSD > 0 {
		projected := result.CostEstimateUSD + provider.HunyuanRapidCostUSD
		if projected > opts.MaxCostUSD {
			return finalize(cliresult.ConfigError(fmt.Errorf(
				"--max-cost cap of $%.2f would be exceeded by Hunyuan3D call (cumulative $%.2f)",
				opts.MaxCostUSD, projected)))
		}
	}

	// Stage 2: 3D model
	modStart := time.Now()
	if !opts.JSON {
		display.StageStart("Generating 3D model via Hunyuan3D...")
	}
	modelPath, err := provider.GenerateModel(ctx, hunyuanSecretID, hunyuanSecretKey, imgResult.Path, imgResult.URL, run.Dir())
	modElapsed := time.Since(modStart)
	if err != nil {
		_ = run.MarkStage("model3d", runs.StatusFailed, modElapsed, err)
		_ = run.SetStatus(runs.StatusFailed)
		result.Stages["model3d"] = cliresult.StageBrief{Status: cliresult.StatusFailed, ElapsedMs: modElapsed.Milliseconds()}
		return finalize(cliresult.StageError("model3d", err))
	}
	_ = run.MarkStage("model3d", runs.StatusComplete, modElapsed, nil)
	_ = run.SetStageCost("model3d", provider.HunyuanRapidCostUSD)
	result.Stages["model3d"] = cliresult.StageBrief{
		Status:          cliresult.StatusComplete,
		ElapsedMs:       modElapsed.Milliseconds(),
		CostEstimateUSD: provider.HunyuanRapidCostUSD,
	}
	result.CostEstimateUSD += provider.HunyuanRapidCostUSD
	if !opts.JSON {
		display.StageDone(modStart)
		if stat, statErr := os.Stat(modelPath); statErr == nil {
			display.StageInfo(fmt.Sprintf("Raw model: %.1f MB", float64(stat.Size())/(1024*1024)))
		}
	}

	// Copy raw → final inside the run dir, and to the user-supplied output path.
	modelData, err := os.ReadFile(modelPath)
	if err != nil {
		return finalize(cliresult.StageError("io", fmt.Errorf("reading model: %w", err)))
	}
	finalInRun := filepath.Join(run.Dir(), "model_final.glb")
	if err := os.WriteFile(finalInRun, modelData, 0o644); err != nil {
		return finalize(cliresult.StageError("io", fmt.Errorf("writing final model in run dir: %w", err)))
	}
	if err := os.WriteFile(opts.OutputPath, modelData, 0o644); err != nil {
		return finalize(cliresult.StageError("io", fmt.Errorf("writing output: %w", err)))
	}

	_ = run.SetStatus(runs.StatusComplete)

	absPath, absErr := filepath.Abs(opts.OutputPath)
	if absErr != nil {
		absPath = opts.OutputPath
	}
	result.Output = absPath

	if !opts.JSON {
		outStat, _ := os.Stat(absPath)
		sizeMB := float64(0)
		if outStat != nil {
			sizeMB = float64(outStat.Size()) / (1024 * 1024)
		}
		display.Success(absPath, sizeMB)
	}

	return finalize(nil)
}
