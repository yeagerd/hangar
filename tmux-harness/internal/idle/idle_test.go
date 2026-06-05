package idle

import (
	"context"
	"testing"
	"time"

	"github.com/articulant/tmux-harness/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCapture returns a fixed pane string or error.
type mockCapture struct {
	content string
	err     error
}

func (m *mockCapture) CapturePane(_ string, _ int) (string, error) {
	return m.content, m.err
}

// mockUpdater records calls; optionally mutates the workspace in-place (stored externally).
type mockUpdater struct {
	applied []func(*store.Workspace)
	err     error
}

func (m *mockUpdater) Update(_ string, apply func(*store.Workspace)) error {
	if m.err != nil {
		return m.err
	}
	m.applied = append(m.applied, apply)
	return nil
}

func newWS(lastHash string, lastChanged time.Time) store.Workspace {
	return store.Workspace{
		ID:              "ws-1",
		Name:            "test",
		TmuxSession:     "harness-test",
		Status:          store.StatusActive,
		LastCaptureHash: lastHash,
		LastChangedAt:   lastChanged,
	}
}

func TestCheck_FirstCall_AlwaysBusy(t *testing.T) {
	// Hash is empty on first call → treated as a change → busy.
	cap := &mockCapture{content: "some output\n"}
	upd := &mockUpdater{}
	ws := newWS("", time.Now().Add(-10*time.Second))

	status, err := Check(context.Background(), ws, cap, upd, 5000)
	require.NoError(t, err)
	assert.False(t, status.Idle)
	assert.Len(t, upd.applied, 1, "update should have been called")
}

func TestCheck_HashChanged_Busy(t *testing.T) {
	cap := &mockCapture{content: "new output\n"}
	upd := &mockUpdater{}
	ws := newWS(hashContent("old output\n"), time.Now().Add(-10*time.Second))

	status, err := Check(context.Background(), ws, cap, upd, 5000)
	require.NoError(t, err)
	assert.False(t, status.Idle)
	assert.Len(t, upd.applied, 1, "update should have been called on hash change")
}

func TestCheck_HashSame_BelowThreshold_Busy(t *testing.T) {
	content := "stable output\n"
	h := hashContent(content)
	cap := &mockCapture{content: content}
	upd := &mockUpdater{}
	// Changed 1 second ago, threshold is 5000 ms → not idle.
	ws := newWS(h, time.Now().Add(-1*time.Second))

	status, err := Check(context.Background(), ws, cap, upd, 5000)
	require.NoError(t, err)
	assert.False(t, status.Idle)
	assert.Empty(t, upd.applied, "no update expected when hash unchanged")
}

func TestCheck_HashSame_AboveThreshold_Idle(t *testing.T) {
	content := "stable output\n"
	h := hashContent(content)
	cap := &mockCapture{content: content}
	upd := &mockUpdater{}
	// Changed 10 seconds ago, threshold is 5000 ms → idle.
	ws := newWS(h, time.Now().Add(-10*time.Second))

	status, err := Check(context.Background(), ws, cap, upd, 5000)
	require.NoError(t, err)
	assert.True(t, status.Idle)
	assert.Equal(t, int64(5000), status.ThresholdMs)
	assert.Empty(t, upd.applied)
}

func TestCheck_CaptureError(t *testing.T) {
	cap := &mockCapture{err: assert.AnError}
	upd := &mockUpdater{}
	ws := newWS("", time.Now())

	_, err := Check(context.Background(), ws, cap, upd, 5000)
	assert.Error(t, err)
}

func TestCheck_UpdateError(t *testing.T) {
	cap := &mockCapture{content: "new\n"}
	upd := &mockUpdater{err: assert.AnError}
	ws := newWS("different-hash", time.Now())

	_, err := Check(context.Background(), ws, cap, upd, 5000)
	assert.Error(t, err)
}

func TestLooksIdle(t *testing.T) {
	assert.True(t, looksIdle("doing stuff\n> ", "> "))
	assert.False(t, looksIdle("doing stuff\nprocessing...", "> "))
	assert.False(t, looksIdle("", "> "))
}

func TestCheckWithPromptHeuristic_TiebreakerIdle(t *testing.T) {
	// Hash stable, elapsed is at 85% of threshold, but prompt looks idle → idle.
	content := "some work done\n> "
	h := hashContent(content)
	cap := &mockCapture{content: content}
	upd := &mockUpdater{}
	thresholdMs := int64(5000)
	// 85% of 5000ms = 4250ms elapsed.
	ws := newWS(h, time.Now().Add(-4250*time.Millisecond))

	status, err := CheckWithPromptHeuristic(context.Background(), ws, cap, upd, thresholdMs, "> ")
	require.NoError(t, err)
	assert.True(t, status.Idle)
}

func TestCheckWithPromptHeuristic_TiebreakerBusy(t *testing.T) {
	// Hash stable, elapsed is at 85% of threshold, prompt does NOT look idle → not idle.
	content := "processing..."
	h := hashContent(content)
	cap := &mockCapture{content: content}
	upd := &mockUpdater{}
	thresholdMs := int64(5000)
	ws := newWS(h, time.Now().Add(-4250*time.Millisecond))

	status, err := CheckWithPromptHeuristic(context.Background(), ws, cap, upd, thresholdMs, "> ")
	require.NoError(t, err)
	assert.False(t, status.Idle)
}
