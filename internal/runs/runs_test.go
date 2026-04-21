package runs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

func TestGenerateIDFormat(t *testing.T) {
	id, err := GenerateID(time.Date(2026, 4, 19, 13, 45, 22, 0, time.Local))
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}
	want := regexp.MustCompile(`^2026-04-19_134522_[0-9a-f]{6}$`)
	if !want.MatchString(id) {
		t.Errorf("id %q does not match expected format", id)
	}
}

func TestGenerateIDUnique(t *testing.T) {
	now := time.Now()
	a, _ := GenerateID(now)
	b, _ := GenerateID(now)
	if a == b {
		t.Errorf("expected distinct IDs from same timestamp, got %q twice", a)
	}
}

func TestNewCreatesDistinctDirs(t *testing.T) {
	root := t.TempDir()
	r1, err := New(root, "prompt one", "out1.glb")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	r2, err := New(root, "prompt two", "out2.glb")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if r1.Meta.RunID == r2.Meta.RunID {
		t.Fatalf("expected distinct run IDs, got %q twice", r1.Meta.RunID)
	}
	if r1.Dir() == r2.Dir() {
		t.Fatalf("expected distinct dirs, got %q twice", r1.Dir())
	}
	for _, d := range []string{r1.Dir(), r2.Dir()} {
		if _, err := os.Stat(filepath.Join(d, "meta.json")); err != nil {
			t.Errorf("expected meta.json in %s: %v", d, err)
		}
		if _, err := os.Stat(filepath.Join(d, "prompt.txt")); err != nil {
			t.Errorf("expected prompt.txt in %s: %v", d, err)
		}
	}
}

func TestMetaRoundTrip(t *testing.T) {
	root := t.TempDir()
	r, err := New(root, "test prompt", "out.glb")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.MarkStage("image", StatusComplete, 1234*time.Millisecond, nil); err != nil {
		t.Fatalf("MarkStage: %v", err)
	}
	if err := r.SetStatus(StatusComplete); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(r.Dir(), "meta.json"))
	if err != nil {
		t.Fatalf("read meta.json: %v", err)
	}
	var got Meta
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.RunID != r.Meta.RunID {
		t.Errorf("RunID: got %q want %q", got.RunID, r.Meta.RunID)
	}
	if got.Prompt != "test prompt" {
		t.Errorf("Prompt: got %q", got.Prompt)
	}
	if got.Status != StatusComplete {
		t.Errorf("Status: got %q want %q", got.Status, StatusComplete)
	}
	if got.Stages["image"].Status != StatusComplete {
		t.Errorf("image stage status: got %q want %q", got.Stages["image"].Status, StatusComplete)
	}
	if got.Stages["image"].ElapsedMs != 1234 {
		t.Errorf("image elapsed_ms: got %d want 1234", got.Stages["image"].ElapsedMs)
	}
	if got.Stages["model3d"].Status != StatusPending {
		t.Errorf("model3d should still be pending: got %q", got.Stages["model3d"].Status)
	}
}

func TestValidateID(t *testing.T) {
	cases := []struct {
		id    string
		valid bool
	}{
		{"2026-04-19_134522_a3f8c2", true},
		{"2026-04-19_134522_A3F8C2", false},  // uppercase hex rejected
		{"2026-04-19_134522_a3f8c", false},    // 5 hex chars
		{"2026-04-19_134522_a3f8c2x", false},  // trailing junk
		{"../../etc/passwd", false},           // path traversal attempt
		{"", false},
		{"2026-04-19_134522", false},          // missing hex
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			err := ValidateID(tc.id)
			if (err == nil) != tc.valid {
				t.Errorf("ValidateID(%q): err=%v, valid=%v", tc.id, err, tc.valid)
			}
		})
	}
}

func TestRead(t *testing.T) {
	root := t.TempDir()
	r, err := New(root, "test prompt", "out.glb")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := r.MarkStage("image", StatusComplete, 100*time.Millisecond, nil); err != nil {
		t.Fatal(err)
	}

	got, err := Read(root, r.Meta.RunID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.RunID != r.Meta.RunID {
		t.Errorf("RunID: got %q want %q", got.RunID, r.Meta.RunID)
	}
	if got.Prompt != "test prompt" {
		t.Errorf("Prompt: got %q", got.Prompt)
	}
	if got.Stages["image"].Status != StatusComplete {
		t.Errorf("image stage: got %q", got.Stages["image"].Status)
	}
}

func TestReadInvalidID(t *testing.T) {
	_, err := Read(t.TempDir(), "../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path-traversal id")
	}
}

func TestReadMissingRun(t *testing.T) {
	_, err := Read(t.TempDir(), "2026-04-19_134522_a3f8c2")
	if err == nil {
		t.Fatal("expected error for nonexistent run")
	}
}

func TestSetParentRunID(t *testing.T) {
	root := t.TempDir()
	r, err := New(root, "p", "out.glb")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetParentRunID("2026-04-18_120000_aabbcc"); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root, r.Meta.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ParentRunID != "2026-04-18_120000_aabbcc" {
		t.Errorf("ParentRunID: got %q", got.ParentRunID)
	}
}

func TestMarkStageWithError(t *testing.T) {
	root := t.TempDir()
	r, err := New(root, "p", "out.glb")
	if err != nil {
		t.Fatal(err)
	}
	testErr := os.ErrNotExist
	if err := r.MarkStage("model3d", StatusFailed, 500*time.Millisecond, testErr); err != nil {
		t.Fatal(err)
	}
	if got := r.Meta.Stages["model3d"].Error; got != testErr.Error() {
		t.Errorf("Error: got %q want %q", got, testErr.Error())
	}
}
