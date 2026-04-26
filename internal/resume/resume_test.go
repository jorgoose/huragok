package resume

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorgoose/huragok/internal/cliresult"
	"github.com/jorgoose/huragok/internal/runs"
)

func TestResumeUnsupportedStage(t *testing.T) {
	_, err := Run(context.Background(), Options{
		ParentRunID: "2026-04-19_134522_a3f8c2",
		Stage:       "image",
		OutputPath:  "out.glb",
		WorkDir:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for unsupported stage")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) || pe.Code != cliresult.ExitConfig {
		t.Errorf("expected ExitConfig, got %T %v", err, err)
	}
}

func TestResumeInvalidParentID(t *testing.T) {
	_, err := Run(context.Background(), Options{
		ParentRunID: "../../etc/passwd",
		Stage:       "model3d",
		OutputPath:  "out.glb",
		WorkDir:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for invalid parent id")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) || pe.Code != cliresult.ExitConfig {
		t.Errorf("expected ExitConfig, got %T %v", err, err)
	}
}

func TestResumeNonexistentParent(t *testing.T) {
	_, err := Run(context.Background(), Options{
		ParentRunID: "2026-04-19_134522_a3f8c2",
		Stage:       "model3d",
		OutputPath:  "out.glb",
		WorkDir:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for nonexistent parent run")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) || pe.Code != cliresult.ExitConfig {
		t.Errorf("expected ExitConfig, got %T %v", err, err)
	}
}

func TestResumeParentMissingConcept(t *testing.T) {
	root := t.TempDir()
	parent, err := runs.New(root, "test", "out.glb")
	if err != nil {
		t.Fatalf("creating parent run: %v", err)
	}
	// Parent exists but no concept.png

	_, err = Run(context.Background(), Options{
		ParentRunID: parent.Meta.RunID,
		Stage:       "model3d",
		OutputPath:  "out.glb",
		WorkDir:     root,
	})
	if err == nil {
		t.Fatal("expected error for parent without concept image")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) || pe.Code != cliresult.ExitConfig {
		t.Errorf("expected ExitConfig, got %T %v", err, err)
	}
	if !strings.Contains(err.Error(), "concept image") {
		t.Errorf("error should mention concept image: %v", err)
	}
}

func TestResumeMaxCostViolation(t *testing.T) {
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "fake-id")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "fake-key")

	root := t.TempDir()
	parent, err := runs.New(root, "test", "out.glb")
	if err != nil {
		t.Fatalf("creating parent run: %v", err)
	}
	if err := os.WriteFile(filepath.Join(parent.Dir(), "concept.png"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = Run(context.Background(), Options{
		ParentRunID: parent.Meta.RunID,
		Stage:       "model3d",
		OutputPath:  "out.glb",
		WorkDir:     root,
		MaxCostUSD:  0.05,
	})
	if err == nil {
		t.Fatal("expected max-cost cap error")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *cliresult.PipelineError, got %T", err)
	}
	if pe.Code != cliresult.ExitConfig {
		t.Errorf("Code: got %d want %d", pe.Code, cliresult.ExitConfig)
	}
	if !strings.Contains(err.Error(), "max-cost") {
		t.Errorf("error should mention max-cost: %v", err)
	}
}

func TestResumeMissingHunyuanCreds(t *testing.T) {
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "")

	root := t.TempDir()
	parent, err := runs.New(root, "test", "out.glb")
	if err != nil {
		t.Fatalf("creating parent run: %v", err)
	}
	// Give parent a concept image
	if err := os.WriteFile(filepath.Join(parent.Dir(), "concept.png"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = Run(context.Background(), Options{
		ParentRunID: parent.Meta.RunID,
		Stage:       "model3d",
		OutputPath:  "out.glb",
		WorkDir:     root,
	})
	if err == nil {
		t.Fatal("expected error for missing Hunyuan creds")
	}
	if !strings.Contains(err.Error(), "HURAGOK_HUNYUAN_SECRET_ID") {
		t.Errorf("error should mention missing Hunyuan key: %v", err)
	}
}
