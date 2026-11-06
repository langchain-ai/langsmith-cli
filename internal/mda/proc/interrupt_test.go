package proc

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOnlyASecondSignalForcesAnExit(t *testing.T) {
	var requested atomic.Bool
	cancelled, forced := 0, 0
	cancel := func() { cancelled++ }
	force := func() { forced++ }

	onSignal(&requested, cancel, force)
	assert.True(t, requested.Load())
	assert.Equal(t, 1, cancelled)
	assert.Equal(t, 0, forced)

	onSignal(&requested, cancel, force)
	assert.Equal(t, 1, cancelled)
	assert.Equal(t, 1, forced)
}

func TestRequestedInterruptDoesNotSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	started := time.Now()

	SleepUntilInterrupted(ctx, 30*time.Second)

	assert.Less(t, time.Since(started), time.Second)
}

func TestPendingInterruptSleepsForTheRequestedDuration(t *testing.T) {
	total := 150 * time.Millisecond
	started := time.Now()

	SleepUntilInterrupted(t.Context(), total)

	assert.GreaterOrEqual(t, time.Since(started), total)
}

func TestPauseFailsOnlyOnceRequested(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	assert.NoError(t, Pause(t.Context(), time.Millisecond))
	assert.ErrorIs(t, Pause(ctx, time.Millisecond), context.Canceled)
}
