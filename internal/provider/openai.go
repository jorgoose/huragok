package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	openai "github.com/sashabaranov/go-openai"
)

// GenerateImage calls OpenAI's image generation API and saves the result to outPath.
// Returns the path to the saved image.
func GenerateImage(ctx context.Context, apiKey, prompt, outDir string) (string, error) {
	client := openai.NewClient(apiKey)

	resp, err := client.CreateImage(ctx, openai.ImageRequest{
		Prompt:         prompt,
		Model:          openai.CreateImageModelDallE3,
		N:              1,
		Size:           openai.CreateImageSize1024x1024,
		ResponseFormat: openai.CreateImageResponseFormatURL,
	})
	if err != nil {
		return "", fmt.Errorf("openai image generation: %w", err)
	}

	if len(resp.Data) == 0 {
		return "", fmt.Errorf("openai returned no images")
	}

	imgURL := resp.Data[0].URL
	if imgURL == "" {
		// Fall back to base64 if URL is empty
		b64 := resp.Data[0].B64JSON
		if b64 == "" {
			return "", fmt.Errorf("openai returned neither URL nor base64 data")
		}
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return "", fmt.Errorf("decoding base64 image: %w", err)
		}
		outPath := filepath.Join(outDir, "concept.png")
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return "", fmt.Errorf("writing image: %w", err)
		}
		return outPath, nil
	}

	// Download image from URL
	httpResp, err := http.Get(imgURL)
	if err != nil {
		return "", fmt.Errorf("downloading image: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading image: HTTP %d", httpResp.StatusCode)
	}

	outPath := filepath.Join(outDir, "concept.png")
	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating image file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, httpResp.Body); err != nil {
		return "", fmt.Errorf("saving image: %w", err)
	}

	return outPath, nil
}
