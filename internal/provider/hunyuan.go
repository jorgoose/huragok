package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go-intl-en/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go-intl-en/tencentcloud/common/profile"
	hunyuan "github.com/tencentcloud/tencentcloud-sdk-go-intl-en/tencentcloud/hunyuan/v20230901"
)

// GenerateModel sends an image to Hunyuan3D Pro via Tencent Cloud and returns the path to the generated .glb file.
func GenerateModel(ctx context.Context, secretID, secretKey, imagePath, outDir string) (string, error) {
	// Read and base64-encode the input image
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("reading image %s: %w", imagePath, err)
	}
	b64Image := base64.StdEncoding.EncodeToString(imgData)

	// Create Tencent Cloud client — hunyuan service, international endpoint, ap-singapore region
	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "hunyuan.intl.tencentcloudapi.com"

	client, err := hunyuan.NewClient(credential, "ap-singapore", cpf)
	if err != nil {
		return "", fmt.Errorf("creating hunyuan client: %w", err)
	}

	// Submit the 3D generation job
	request := hunyuan.NewSubmitHunyuanTo3DProJobRequest()
	request.ImageBase64 = common.StringPtr(b64Image)

	response, err := client.SubmitHunyuanTo3DProJobWithContext(ctx, request)
	if err != nil {
		return "", fmt.Errorf("submitting hunyuan3d job: %w", err)
	}

	if response.Response == nil || response.Response.JobId == nil || *response.Response.JobId == "" {
		return "", fmt.Errorf("hunyuan3d returned empty job ID")
	}

	jobID := *response.Response.JobId

	// Poll for completion
	glbURL, err := pollForResult(ctx, client, jobID)
	if err != nil {
		return "", err
	}

	// Download the .glb file
	data, err := downloadResult(ctx, glbURL)
	if err != nil {
		return "", err
	}

	outPath := filepath.Join(outDir, "model_raw.glb")
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing model: %w", err)
	}

	return outPath, nil
}

func pollForResult(ctx context.Context, client *hunyuan.Client, jobID string) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("cancelled")
		case <-timeout:
			return "", fmt.Errorf("hunyuan3d job timed out after 5 minutes")
		case <-ticker.C:
			request := hunyuan.NewQueryHunyuanTo3DProJobRequest()
			request.JobId = common.StringPtr(jobID)

			resp, err := client.QueryHunyuanTo3DProJobWithContext(ctx, request)
			if err != nil {
				return "", fmt.Errorf("polling hunyuan3d job: %w", err)
			}

			if resp.Response == nil || resp.Response.Status == nil {
				continue
			}

			switch *resp.Response.Status {
			case "DONE":
				for _, f := range resp.Response.ResultFile3Ds {
					if f.Url != nil && *f.Url != "" {
						return *f.Url, nil
					}
				}
				return "", fmt.Errorf("hunyuan3d job completed but no result files returned")

			case "FAIL":
				errMsg := "unknown error"
				if resp.Response.ErrorMessage != nil {
					errMsg = *resp.Response.ErrorMessage
				}
				return "", fmt.Errorf("hunyuan3d job failed: %s", errMsg)

			case "WAIT", "RUN":
				// Still processing
			}
		}
	}
}

func downloadResult(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading model: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading model data: %w", err)
	}

	return data, nil
}
