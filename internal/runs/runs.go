package runs

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	StatusPending  = "pending"
	StatusRunning  = "running"
	StatusComplete = "complete"
	StatusFailed   = "failed"
)

type Meta struct {
	RunID      string                 `json:"run_id"`
	CreatedAt  time.Time              `json:"created_at"`
	Prompt     string                 `json:"prompt"`
	OutputPath string                 `json:"output_path"`
	Status     string                 `json:"status"`
	Stages     map[string]StageResult `json:"stages"`
}

type StageResult struct {
	Status    string `json:"status"`
	ElapsedMs int64  `json:"elapsed_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Run struct {
	Meta Meta
	dir  string
}

// New creates a run directory under rootDir/runs/<id>/ and writes an initial meta.json.
func New(rootDir, prompt, outputPath string) (*Run, error) {
	id, err := GenerateID(time.Now())
	if err != nil {
		return nil, fmt.Errorf("generating run id: %w", err)
	}
	dir := filepath.Join(rootDir, "runs", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating run dir: %w", err)
	}

	abs, err := filepath.Abs(outputPath)
	if err != nil {
		abs = outputPath
	}

	r := &Run{
		dir: dir,
		Meta: Meta{
			RunID:      id,
			CreatedAt:  time.Now(),
			Prompt:     prompt,
			OutputPath: abs,
			Status:     StatusRunning,
			Stages: map[string]StageResult{
				"image":   {Status: StatusPending},
				"model3d": {Status: StatusPending},
			},
		},
	}

	if err := r.WriteMeta(); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "prompt.txt"), []byte(prompt), 0o644); err != nil {
		return nil, fmt.Errorf("writing prompt.txt: %w", err)
	}
	return r, nil
}

func (r *Run) Dir() string { return r.dir }

// MarkStage updates one stage's status and persists meta.json.
func (r *Run) MarkStage(name, status string, elapsed time.Duration, err error) error {
	stage := r.Meta.Stages[name]
	stage.Status = status
	stage.ElapsedMs = elapsed.Milliseconds()
	if err != nil {
		stage.Error = err.Error()
	}
	r.Meta.Stages[name] = stage
	return r.WriteMeta()
}

// SetStatus updates the overall run status and persists meta.json.
func (r *Run) SetStatus(status string) error {
	r.Meta.Status = status
	return r.WriteMeta()
}

func (r *Run) WriteMeta() error {
	data, err := json.MarshalIndent(r.Meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling meta.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(r.dir, "meta.json"), data, 0o644); err != nil {
		return fmt.Errorf("writing meta.json: %w", err)
	}
	return nil
}

// GenerateID returns a unique run ID of form YYYY-MM-DD_HHMMSS_<6-hex>.
// Local time is used since runs are inherently per-machine.
func GenerateID(t time.Time) (string, error) {
	buf := make([]byte, 3)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s", t.Format("2006-01-02_150405"), hex.EncodeToString(buf)), nil
}
