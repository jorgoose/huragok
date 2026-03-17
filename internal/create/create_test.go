package create

import (
	"context"
	"testing"
)

func TestRunMissingOpenAIKey(t *testing.T) {
	t.Setenv("HURAGOK_OPENAI_KEY", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "")

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
	t.Setenv("HURAGOK_HUNYUAN_SECRET_ID", "")
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "")

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
	t.Setenv("HURAGOK_HUNYUAN_SECRET_KEY", "")

	err := Run(context.Background(), "test prompt", "output.glb")
	if err == nil {
		t.Fatal("expected error when HURAGOK_HUNYUAN_SECRET_KEY is missing")
	}
	if got := err.Error(); got != "HURAGOK_HUNYUAN_SECRET_KEY environment variable is required" {
		t.Errorf("unexpected error: %s", got)
	}
}
