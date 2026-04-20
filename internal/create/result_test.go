package create

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, ExitSuccess},
		{"context canceled", context.Canceled, ExitCanceled},
		{"unauthorized", errors.New("openai: 401 Unauthorized"), ExitConfig},
		{"invalid api key", errors.New("Invalid API key provided"), ExitConfig},
		{"timeout", errors.New("Post \"https://...\": context deadline exceeded"), ExitNetwork},
		{"rate limit", errors.New("429 Too Many Requests"), ExitNetwork},
		{"server error", errors.New("503 service unavailable"), ExitNetwork},
		{"connection refused", errors.New("dial tcp: connection refused"), ExitNetwork},
		{"hunyuan FAIL", errors.New("hunyuan3d job failed: bad image"), ExitStage},
		{"empty response", errors.New("openai returned no images"), ExitStage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classify(tc.err); got != tc.want {
				t.Errorf("classify(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestPipelineErrorUnwrap(t *testing.T) {
	inner := errors.New("inner")
	pe := &PipelineError{Code: ExitNetwork, Stage: "image", Err: inner}
	if !errors.Is(pe, inner) {
		t.Errorf("errors.Is should find inner via Unwrap")
	}
	if pe.Error() != "inner" {
		t.Errorf("Error(): got %q want %q", pe.Error(), "inner")
	}
}

func TestResultWriteJSONShape(t *testing.T) {
	r := &Result{
		RunID:  "2026-04-19_134522_a3f8c2",
		Status: StatusComplete,
		Stages: map[string]StageBrief{
			"image":   {Status: StatusComplete, ElapsedMs: 13300},
			"model3d": {Status: StatusComplete, ElapsedMs: 65200},
		},
		Output:         "/abs/out.glb",
		ElapsedSeconds: 78.5,
	}
	var buf bytes.Buffer
	if err := r.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	for _, key := range []string{"run_id", "status", "stages", "output", "elapsed_seconds"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
	if _, ok := got["error"]; ok {
		t.Errorf("error key should be omitted on success")
	}
}

func TestResultWriteJSONErrorEnvelope(t *testing.T) {
	r := &Result{
		RunID:  "2026-04-19_134522_a3f8c2",
		Status: StatusFailed,
		Error: &ResultError{
			Stage:   "image",
			Message: "openai returned no images",
		},
		ElapsedSeconds: 4.2,
	}
	var buf bytes.Buffer
	if err := r.WriteJSON(&buf); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object: %v", got["error"])
	}
	if errObj["stage"] != "image" {
		t.Errorf("error.stage: got %v", errObj["stage"])
	}
	if !contains(fmt.Sprint(errObj["message"]), "openai") {
		t.Errorf("error.message: got %v", errObj["message"])
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
