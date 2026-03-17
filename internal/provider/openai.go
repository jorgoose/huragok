package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const maxImageRetries = 3

// ImageResult holds both the local file path and the remote URL of the generated image.
type ImageResult struct {
	Path string // Local file path
	URL  string // Remote URL (may be empty if base64 response)
}

// GenerateImage calls OpenAI's image generation API and saves the result to outDir.
// Returns the local path and the remote URL.
func GenerateImage(ctx context.Context, apiKey, prompt, outDir string) (*ImageResult, error) {
	client := openai.NewClient(apiKey)

	// Append 3D-optimized suffix for cleaner model generation
	enhancedPrompt := prompt + ", single object, centered, isolated on plain white background, product photography style, no text"

	var resp openai.ImageResponse
	var lastErr error
	for attempt := 0; attempt < maxImageRetries; attempt++ {
		resp, lastErr = client.CreateImage(ctx, openai.ImageRequest{
			Prompt:         enhancedPrompt,
			Model:          openai.CreateImageModelDallE3,
			N:              1,
			Size:           openai.CreateImageSize1024x1024,
			ResponseFormat: openai.CreateImageResponseFormatURL,
		})
		if lastErr == nil {
			break
		}
		// Retry only on content filter errors (non-deterministic)
		if !strings.Contains(lastErr.Error(), "content") {
			return nil, fmt.Errorf("openai image generation: %w", lastErr)
		}
		if attempt < maxImageRetries-1 {
			fmt.Printf(" (content filter, retrying %d/%d...)", attempt+2, maxImageRetries)
			time.Sleep(2 * time.Second)
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("openai image generation: %w", lastErr)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai returned no images")
	}

	imgURL := resp.Data[0].URL
	if imgURL == "" {
		// Fall back to base64 if URL is empty
		b64 := resp.Data[0].B64JSON
		if b64 == "" {
			return nil, fmt.Errorf("openai returned neither URL nor base64 data")
		}
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("decoding base64 image: %w", err)
		}
		outPath := filepath.Join(outDir, "concept.png")
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return nil, fmt.Errorf("writing image: %w", err)
		}
		return &ImageResult{Path: outPath}, nil
	}

	// Download image from URL
	httpResp, err := http.Get(imgURL)
	if err != nil {
		return nil, fmt.Errorf("downloading image: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading image: HTTP %d", httpResp.StatusCode)
	}

	outPath := filepath.Join(outDir, "concept.png")
	f, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("creating image file: %w", err)
	}

	if _, err := io.Copy(f, httpResp.Body); err != nil {
		f.Close()
		os.Remove(outPath)
		return nil, fmt.Errorf("saving image: %w", err)
	}
	f.Close()

	return &ImageResult{Path: outPath, URL: imgURL}, nil
}
