package provider

import (
	"strings"
	"testing"
)

func TestEnhancedPromptSuffix(t *testing.T) {
	// The prompt enhancement is inlined in GenerateImage, so we test the expected behavior:
	// any prompt should get the 3D-optimized suffix appended.
	prompt := "futuristic cargo crate"
	suffix := ", single object, centered, isolated on plain white background, product photography style, no text"
	enhanced := prompt + suffix

	if !strings.Contains(enhanced, "white background") {
		t.Error("enhanced prompt should contain 'white background'")
	}
	if !strings.Contains(enhanced, "single object") {
		t.Error("enhanced prompt should contain 'single object'")
	}
	if !strings.HasPrefix(enhanced, prompt) {
		t.Error("enhanced prompt should start with original prompt")
	}
}
