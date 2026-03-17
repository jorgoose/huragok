package create

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jorgoose/huragok/internal/display"
	"github.com/jorgoose/huragok/internal/provider"
)

// Run executes the full create pipeline: image generation → 3D model generation.
func Run(ctx context.Context, prompt, outputPath string) error {
	// Read API keys from environment
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

	// Create working directory for intermediate artifacts
	workDir := ".huragok"
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("creating work directory: %w", err)
	}

	// Ensure output directory exists
	outDir := filepath.Dir(outputPath)
	if outDir != "" && outDir != "." {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
	}

	display.Header()
	display.Prompt(prompt)

	// Stage 1: Generate concept image via OpenAI
	t := display.StageStart("Generating concept image...")
	imgResult, err := provider.GenerateImage(ctx, openaiKey, prompt, workDir)
	if err != nil {
		display.Error(err.Error())
		return err
	}
	display.StageDone(t)
	display.StageInfo(fmt.Sprintf("Saved → %s", imgResult.Path))
	fmt.Println()

	// Stage 2: Generate 3D model via Hunyuan3D
	t = display.StageStart("Generating 3D model via Hunyuan3D...")
	modelPath, err := provider.GenerateModel(ctx, hunyuanSecretID, hunyuanSecretKey, imgResult.Path, imgResult.URL, workDir)
	if err != nil {
		display.Error(err.Error())
		return err
	}
	display.StageDone(t)

	// Get file size
	stat, err := os.Stat(modelPath)
	if err == nil {
		display.StageInfo(fmt.Sprintf("Raw model: %.1f MB", float64(stat.Size())/(1024*1024)))
	}

	// Copy to output path
	modelData, err := os.ReadFile(modelPath)
	if err != nil {
		return fmt.Errorf("reading model: %w", err)
	}
	if err := os.WriteFile(outputPath, modelData, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

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
