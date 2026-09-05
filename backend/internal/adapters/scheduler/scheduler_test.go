package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fire-lookout/backend/internal/application"
)

type fakePoller struct {
	mu      sync.Mutex
	runs    atomic.Int64
	err     error
	release chan struct{}
}

func (p *fakePoller) PollDue(context.Context) (application.Summary, error) {
	p.runs.Add(1)

	p.mu.Lock()
	release := p.release
	err := p.err
	p.mu.Unlock()

	if release != nil {
		<-release
	}
	return application.Summary{Updated: 1}, err
}

// waitForRuns waits for at least n runs, so tests never depend on a fixed sleep.
func waitForRuns(t *testing.T, poller *fakePoller, n int64) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if poller.runs.Load() >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("only %d runs after waiting, want at least %d", poller.runs.Load(), n)
}

// A fresh process — or a feed added moments ago — must not wait a whole tick.
func TestRunPollsImmediately(t *testing.T) {
	poller := &fakePoller{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go Run(ctx, time.Hour, poller)

	waitForRuns(t, poller, 1)
}

func TestRunKeepsPollingOnEachTick(t *testing.T) {
	poller := &fakePoller{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go Run(ctx, 5*time.Millisecond, poller)

	waitForRuns(t, poller, 3)
}

func TestRunStopsWhenTheContextIsCancelled(t *testing.T) {
	poller := &fakePoller{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		Run(ctx, 5*time.Millisecond, poller)
		close(done)
	}()

	waitForRuns(t, poller, 1)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}

	settled := poller.runs.Load()
	time.Sleep(20 * time.Millisecond)
	if after := poller.runs.Load(); after != settled {
		t.Errorf("polled %d more times after shutdown, want none", after-settled)
	}
}

// One bad run must not kill the loop; the next tick tries again.
func TestRunSurvivesAFailingPoll(t *testing.T) {
	poller := &fakePoller{err: errors.New("database gone")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go Run(ctx, 5*time.Millisecond, poller)

	waitForRuns(t, poller, 3)
}

func TestRunFallsBackToTheDefaultTick(t *testing.T) {
	poller := &fakePoller{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// A zero tick would otherwise panic inside time.NewTicker.
	go Run(ctx, 0, poller)

	waitForRuns(t, poller, 1)
}
