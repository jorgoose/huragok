package runs

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

var runIDPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}_\d{6}_[0-9a-f]{6}$`)

// ValidateID returns nil iff id matches the YYYY-MM-DD_HHMMSS_<6-hex> format.
// Validating before joining into a path prevents traversal via crafted IDs.
func ValidateID(id string) error {
	if !runIDPattern.MatchString(id) {
		return fmt.Errorf("invalid run id %q (expected YYYY-MM-DD_HHMMSS_<6-hex>)", id)
	}
	return nil
}

// Read loads a run's meta.json from disk.
func Read(rootDir, runID string) (*Meta, error) {
	if err := ValidateID(runID); err != nil {
		return nil, err
	}
	metaPath := filepath.Join(rootDir, "runs", runID, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("reading meta.json for run %s: %w", runID, err)
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing meta.json for run %s: %w", runID, err)
	}
	return &meta, nil
}

// RunDir returns the absolute-or-relative path to the run's directory.
// Used by callers that need to read sibling artifacts (e.g., concept.png).
func RunDir(rootDir, runID string) (string, error) {
	if err := ValidateID(runID); err != nil {
		return "", err
	}
	return filepath.Join(rootDir, "runs", runID), nil
}

// List walks rootDir/runs/ and returns the parsed meta.json from every
// validly-named run directory, sorted newest-first by run ID.
// Directories whose meta.json can't be read or parsed are skipped silently —
// runs in flight may be missing the file briefly, and litter shouldn't fail
// the whole listing.
func List(rootDir string) ([]Meta, error) {
	runsDir := filepath.Join(rootDir, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading runs dir: %w", err)
	}
	var out []Meta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if ValidateID(e.Name()) != nil {
			continue
		}
		meta, err := Read(rootDir, e.Name())
		if err != nil {
			continue
		}
		out = append(out, *meta)
	}
	// Run IDs start with the timestamp, so descending lex sort = newest-first.
	sort.Slice(out, func(i, j int) bool { return out[i].RunID > out[j].RunID })
	return out, nil
}

// ListArtifacts returns the file names (not full paths) directly under the
// run's directory.
func ListArtifacts(rootDir, runID string) ([]string, error) {
	dir, err := RunDir(rootDir, runID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading run dir for %s: %w", runID, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

const (
	StatusPending  = "pending"
	StatusRunning  = "running"
	StatusComplete = "complete"
	StatusFailed   = "failed"
)

type Meta struct {
	RunID       string                 `json:"run_id"`
	CreatedAt   time.Time              `json:"created_at"`
	Prompt      string                 `json:"prompt"`
	OutputPath  string                 `json:"output_path"`
	Status      string                 `json:"status"`
	Stages      map[string]StageResult `json:"stages"`
	ImageSource string                 `json:"image_source,omitempty"`  // "openai" | "user"
	ParentRunID string                 `json:"parent_run_id,omitempty"` // set when this run resumes another
}

type StageResult struct {
	Status          string  `json:"status"`
	ElapsedMs       int64   `json:"elapsed_ms,omitempty"`
	Error           string  `json:"error,omitempty"`
	CostEstimateUSD float64 `json:"cost_estimate_usd,omitempty"`
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

// SetImageSource records where the concept image came from ("openai" or "user").
func (r *Run) SetImageSource(s string) error {
	r.Meta.ImageSource = s
	return r.WriteMeta()
}

// SetParentRunID records that this run resumes from another.
func (r *Run) SetParentRunID(id string) error {
	r.Meta.ParentRunID = id
	return r.WriteMeta()
}

// SetStageCost records the estimated USD cost of a stage.
func (r *Run) SetStageCost(name string, cost float64) error {
	stage := r.Meta.Stages[name]
	stage.CostEstimateUSD = cost
	r.Meta.Stages[name] = stage
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
