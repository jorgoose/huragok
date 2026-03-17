package display

import (
	"testing"
	"time"
)

func TestHeaderDoesNotPanic(t *testing.T) {
	Header()
}

func TestPromptDoesNotPanic(t *testing.T) {
	Prompt("test prompt")
}

func TestStageStartAndDone(t *testing.T) {
	start := StageStart("Testing...")
	if start.IsZero() {
		t.Error("StageStart should return a non-zero time")
	}
	StageDone(start)
}

func TestStageInfo(t *testing.T) {
	StageInfo("some info")
}

func TestSuccess(t *testing.T) {
	Success("output.glb", 1.5)
}

func TestError(t *testing.T) {
	Error("something went wrong")
}

func TestStageDoneElapsed(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	// Should not panic with a past start time
	StageDone(start)
}
