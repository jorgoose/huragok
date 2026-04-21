package resume

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
	ParentRunID string
	Stage       string // currently only "model3d" is supported
	OutputPath  string
	JSON        bool
	WorkDir     string
}

// Run re-executes a single stage of a previous run. It creates a NEW run
// directory whose meta.json records parent_run_id, copies the parent's
// concept image into it, and calls the relevant provider stage.
func Run(ctx context.Context, opts Options) (*cliresult.Result, error) {
	started := time.Now()
	result := &cliresult.Result{
		Stages:      map[string]cliresult.StageBrief{},
		ParentRunID: opts.ParentRunID,
	}

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

	if opts.Stage != "model3d" {
		return finalize(cliresult.ConfigError(fmt.Errorf("unsupported resume stage %q (only model3d is supported in v0.2)", opts.Stage)))
	}

	workDir := opts.WorkDir
	if workDir == "" {
		workDir = ".huragok"
	}

	parentMeta, err := runs.Read(workDir, opts.ParentRunID)
	if err != nil {
		return finalize(cliresult.ConfigError(err))
	}

	parentDir, err := runs.RunDir(workDir, opts.ParentRunID)
	if err != nil {
		return finalize(cliresult.ConfigError(err))
	}
	parentConcept := filepath.Join(parentDir, "concept.png")
	if _, err := os.Stat(parentConcept); err != nil {
		return finalize(cliresult.ConfigError(fmt.Errorf("parent run %s has no concept image at %s", opts.ParentRunID, parentConcept)))
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

	run, err := runs.New(workDir, parentMeta.Prompt, opts.OutputPath)
	if err != nil {
		return finalize(cliresult.ConfigError(err))
	}
	_ = run.SetParentRunID(opts.ParentRunID)
	if parentMeta.ImageSource != "" {
		_ = run.SetImageSource(parentMeta.ImageSource)
	}
	result.RunID = run.Meta.RunID

	if !opts.JSON {
		display.Header()
		if parentMeta.Prompt != "" {
			display.Prompt(parentMeta.Prompt)
		}
		display.RunID(run.Meta.RunID)
		display.StageInfo(fmt.Sprintf("Resuming from parent run %s", opts.ParentRunID))
		fmt.Println()
	}

	conceptDst := filepath.Join(run.Dir(), "concept.png")
	src, err := os.ReadFile(parentConcept)
	if err != nil {
		return finalize(cliresult.StageError("io", fmt.Errorf("reading parent concept image: %w", err)))
	}
	if err := os.WriteFile(conceptDst, src, 0o644); err != nil {
		return finalize(cliresult.StageError("io", fmt.Errorf("copying parent concept image: %w", err)))
	}
	_ = run.MarkStage("image", runs.StatusComplete, 0, nil)
	result.Stages["image"] = cliresult.StageBrief{Status: cliresult.StatusComplete, ElapsedMs: 0}

	modStart := time.Now()
	if !opts.JSON {
		display.StageStart("Generating 3D model via Hunyuan3D...")
	}
	modelPath, err := provider.GenerateModel(ctx, hunyuanSecretID, hunyuanSecretKey, conceptDst, "", run.Dir())
	modElapsed := time.Since(modStart)
	if err != nil {
		_ = run.MarkStage("model3d", runs.StatusFailed, modElapsed, err)
		_ = run.SetStatus(runs.StatusFailed)
		result.Stages["model3d"] = cliresult.StageBrief{Status: cliresult.StatusFailed, ElapsedMs: modElapsed.Milliseconds()}
		return finalize(cliresult.StageError("model3d", err))
	}
	_ = run.MarkStage("model3d", runs.StatusComplete, modElapsed, nil)
	result.Stages["model3d"] = cliresult.StageBrief{Status: cliresult.StatusComplete, ElapsedMs: modElapsed.Milliseconds()}
	if !opts.JSON {
		display.StageDone(modStart)
		if stat, statErr := os.Stat(modelPath); statErr == nil {
			display.StageInfo(fmt.Sprintf("Raw model: %.1f MB", float64(stat.Size())/(1024*1024)))
		}
	}

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
