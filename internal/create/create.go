package create

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jorgoose/huragok/internal/display"
	"github.com/jorgoose/huragok/internal/provider"
	"github.com/jorgoose/huragok/internal/runs"
)

// Run executes the full create pipeline: image generation → 3D model generation.
func Run(ctx context.Context, prompt, outputPath string) error {
	openaiKey := os.Getenv("HURAGOK_OPENAI_KEY")
	if openaiKey == "" {
		return fmt.Errorf("HURAGOK_OPENAI_KEY environment variable is required")
	}
	hunyuanSecretID := os.Getenv("HURAGOK_HUNYUAN_SECRET_ID")
	if hunyuanSecretID == "" {
		return fmt.Errorf("HURAGOK_HUNYUAN_SECRET_ID environment variable is required")
	}
	hunyuanSecretKey := os.Getenv("HURAGOK_HUNYUAN_SECRET_KEY")
	if hunyuanSecretKey == "" {
		return fmt.Errorf("HURAGOK_HUNYUAN_SECRET_KEY environment variable is required")
	}

	outDir := filepath.Dir(outputPath)
	if outDir != "" && outDir != "." {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	run, err := runs.New(".huragok", prompt, outputPath)
	if err != nil {
		return err
	}

	display.Header()
	display.Prompt(prompt)
	display.RunID(run.Meta.RunID)

	// Stage 1: concept image
	t := display.StageStart("Generating concept image...")
	imgResult, err := provider.GenerateImage(ctx, openaiKey, prompt, run.Dir())
	if err != nil {
		_ = run.MarkStage("image", runs.StatusFailed, time.Since(t), err)
		_ = run.SetStatus(runs.StatusFailed)
		display.Error(err.Error())
		return err
	}
	_ = run.MarkStage("image", runs.StatusComplete, time.Since(t), nil)
	display.StageDone(t)
	display.StageInfo(fmt.Sprintf("Saved → %s", imgResult.Path))
	fmt.Println()

	// Stage 2: 3D model
	t = display.StageStart("Generating 3D model via Hunyuan3D...")
	modelPath, err := provider.GenerateModel(ctx, hunyuanSecretID, hunyuanSecretKey, imgResult.Path, imgResult.URL, run.Dir())
	if err != nil {
		_ = run.MarkStage("model3d", runs.StatusFailed, time.Since(t), err)
		_ = run.SetStatus(runs.StatusFailed)
		display.Error(err.Error())
		return err
	}
	_ = run.MarkStage("model3d", runs.StatusComplete, time.Since(t), nil)
	display.StageDone(t)

	if stat, err := os.Stat(modelPath); err == nil {
		display.StageInfo(fmt.Sprintf("Raw model: %.1f MB", float64(stat.Size())/(1024*1024)))
	}

	// Copy raw → final inside the run dir, and to the user-supplied output path.
	// final.glb is byte-identical to raw.glb today; the named slot is reserved
	// for postprocessing output once that lands.
	modelData, err := os.ReadFile(modelPath)
	if err != nil {
		return fmt.Errorf("reading model: %w", err)
	}
	finalInRun := filepath.Join(run.Dir(), "model_final.glb")
	if err := os.WriteFile(finalInRun, modelData, 0o644); err != nil {
		return fmt.Errorf("writing final model in run dir: %w", err)
	}
	if err := os.WriteFile(outputPath, modelData, 0o644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	_ = run.SetStatus(runs.StatusComplete)

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		absPath = outputPath
	}
	outStat, _ := os.Stat(absPath)
	sizeMB := float64(0)
	if outStat != nil {
		sizeMB = float64(outStat.Size()) / (1024 * 1024)
	}
	display.Success(absPath, sizeMB)

	return nil
}
