package create

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jorgoose/huragok/internal/cliresult"
)

func TestRunMissingOpenAIKey(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "")

	_, err := Run(context.Background(), Options{Prompt: "test prompt", OutputPath: "output.glb", WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error when HURAGOK_OPENAI_KEY is missing")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *cliresult.PipelineError, got %T: %v", err, err)
	}
	if pe.Code != cliresult.ExitConfig {
		t.Errorf("Code: got %d want %d", pe.Code, cliresult.ExitConfig)
	}
	if !strings.Contains(err.Error(), "HURAGOK_OPENAI_KEY") {
		t.Errorf("error should mention HURAGOK_OPENAI_KEY: %v", err)
	}
}

func TestRunMissingHunyuanSecretID(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "fake-key")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "")

	_, err := Run(context.Background(), Options{Prompt: "test prompt", OutputPath: "output.glb", WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error when HURAGOK_HUNYUAN_SECRET_ID is missing")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *cliresult.PipelineError, got %T: %v", err, err)
	}
	if pe.Code != cliresult.ExitConfig {
		t.Errorf("Code: got %d want %d", pe.Code, cliresult.ExitConfig)
	}
	if !strings.Contains(err.Error(), "HURAGOK_HUNYUAN_SECRET_ID") {
		t.Errorf("error should mention HURAGOK_HUNYUAN_SECRET_ID: %v", err)
	}
}

func TestRunMissingHunyuanSecretKey(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "fake-key")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "fake-id")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "")

	_, err := Run(context.Background(), Options{Prompt: "test prompt", OutputPath: "output.glb", WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error when HURAGOK_HUNYUAN_SECRET_KEY is missing")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *cliresult.PipelineError, got %T: %v", err, err)
	}
	if pe.Code != cliresult.ExitConfig {
		t.Errorf("Code: got %d want %d", pe.Code, cliresult.ExitConfig)
	}
	if !strings.Contains(err.Error(), "HURAGOK_HUNYUAN_SECRET_KEY") {
		t.Errorf("error should mention HURAGOK_HUNYUAN_SECRET_KEY: %v", err)
	}
}

func TestRunConfigErrorReturnsResult(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "")

	result, err := Run(context.Background(), Options{Prompt: "p", OutputPath: "o.glb", WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error")
	}
	if result == nil {
		t.Fatal("expected non-nil result on config error")
	}
	if result.Status != cliresult.StatusFailed {
		t.Errorf("Status: got %q want %q", result.Status, cliresult.StatusFailed)
	}
	if result.Error == nil {
		t.Fatal("expected non-nil result.Error")
	}
	if result.Error.Stage != "config" {
		t.Errorf("Error.Stage: got %q want config", result.Error.Stage)
	}
	if !strings.Contains(result.Error.Message, "HURAGOK_OPENAI_KEY") {
		t.Errorf("Error.Message: got %q", result.Error.Message)
	}
}

func TestRunFromMissingImage(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "fake-id")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "fake-key")

	_, err := Run(context.Background(), Options{
		From:       "/nonexistent/image.png",
		OutputPath: "o.glb",
		WorkDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for missing --from path")
	}
	var pe *cliresult.PipelineError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *cliresult.PipelineError, got %T", err)
	}
	if pe.Code != cliresult.ExitConfig {
		t.Errorf("Code: got %d want %d", pe.Code, cliresult.ExitConfig)
	}
	if !strings.Contains(err.Error(), "/nonexistent/image.png") {
		t.Errorf("error should reference the bad path: %v", err)
	}
}

func TestRunFromSkipsOpenAIRequirement(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "fake-id")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "fake-key")

	_, err := Run(context.Background(), Options{
		From:       "create_test.go",
		OutputPath: "o.glb",
		WorkDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected eventual error from Hunyuan with fake creds")
	}
	if strings.Contains(err.Error(), "HURAGOK_OPENAI_KEY") {
		t.Errorf("--from should skip the OpenAI key check, got: %v", err)
	}
}
