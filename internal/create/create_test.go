package create

import (
	"context"
	"os"
	"testing"
)

func TestRunMissingOpenAIKey(t *testing.T) {
	os.Unsetenv("HURAGOK_OPENAI_KEY")
	os.Unsetenv("HURAGOK_HUNYUAN_SECRET_ID")
	os.Unsetenv("HURAGOK_HUNYUAN_SECRET_KEY")

	err := Run(context.Background(), "test prompt", "output.glb")
	if err == nil {
		t.Fatal("expected error when HURAGOK_OPENAI_KEY is missing")
	}
	if got := err.Error(); got != "HURAGOK_OPENAI_KEY environment variable is required" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestRunMissingHunyuanSecretID(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "fake-key")
	os.Unsetenv("HURAGOK_HUNYUAN_SECRET_ID")
	os.Unsetenv("HURAGOK_HUNYUAN_SECRET_KEY")

	err := Run(context.Background(), "test prompt", "output.glb")
	if err == nil {
		t.Fatal("expected error when HURAGOK_HUNYUAN_SECRET_ID is missing")
	}
	if got := err.Error(); got != "HURAGOK_HUNYUAN_SECRET_ID environment variable is required" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestRunMissingHunyuanSecretKey(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "fake-key")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "fake-id")
	os.Unsetenv("HURAGOK_HUNYUAN_SECRET_KEY")

	err := Run(context.Background(), "test prompt", "output.glb")
	if err == nil {
		t.Fatal("expected error when HURAGOK_HUNYUAN_SECRET_KEY is missing")
	}
	if got := err.Error(); got != "HURAGOK_HUNYUAN_SECRET_KEY environment variable is required" {
		t.Errorf("unexpected error: %s", got)
	}
}
