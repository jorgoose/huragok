package create

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const (
	ExitSuccess  = 0
	ExitStage    = 1
	ExitConfig   = 2
	ExitNetwork  = 3
	ExitCanceled = 4
)

const (
	StatusComplete = "complete"
	StatusFailed   = "failed"
)

type Options struct {
	Prompt     string
	OutputPath string
	JSON       bool
	From       string // path to a user-supplied image; bypasses OpenAI
}

type Result struct {
	RunID          string                `json:"run_id,omitempty"`
	Status         string                `json:"status"`
	Stages         map[string]StageBrief `json:"stages,omitempty"`
	Output         string                `json:"output,omitempty"`
	ElapsedSeconds float64               `json:"elapsed_seconds"`
	Error          *ResultError          `json:"error,omitempty"`
}

type StageBrief struct {
	Status    string `json:"status"`
	ElapsedMs int64  `json:"elapsed_ms"`
}

type ResultError struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
}

// WriteJSON writes the result as a single indented JSON object.
func (r *Result) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// PipelineError carries the exit code that main should return.
type PipelineError struct {
	Code  int
	Stage string
	Err   error
}

func (e *PipelineError) Error() string {
	if e.Err == nil {
		return e.Stage
	}
	return e.Err.Error()
}

func (e *PipelineError) Unwrap() error { return e.Err }

// classify maps an underlying API/IO error to an exit code. Heuristic — leans
// on substring matching because providers don't expose typed auth/rate-limit
// errors consistently.
func classify(err error) int {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, context.Canceled) {
		return ExitCanceled
	}
	msg := strings.ToLower(err.Error())
	switch {
	case containsAny(msg, "401", "403", "unauthorized", "invalid api key", "incorrect api key", "authentication"):
		return ExitConfig
	case containsAny(msg, "timeout", "timed out", "connection", "rate limit", "429", "500", "502", "503", "context deadline"):
		return ExitNetwork
	default:
		return ExitStage
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func stageError(stage string, err error) *PipelineError {
	return &PipelineError{Code: classify(err), Stage: stage, Err: err}
}

func configError(err error) *PipelineError {
	return &PipelineError{Code: ExitConfig, Stage: "config", Err: err}
}
