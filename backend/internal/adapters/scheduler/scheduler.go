// Package scheduler is the driving adapter that decides *when* the poller runs. It holds no
// polling logic of its own — it drives the application use case on a clock, the way httpapi
// drives it on a request.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"fire-lookout/backend/internal/application"
)

// DefaultTick is how often the scheduler looks for work. It is not a feed's cadence: each
// feed's own refresh_interval_sec decides whether it is due, so this only bounds how
// promptly a due feed gets picked up.
const DefaultTick = 15 * time.Second

// Poller is the use case the scheduler drives.
type Poller interface {
	PollDue(ctx context.Context) (application.Summary, error)
}

// Run polls once immediately, then every tick until ctx is cancelled.
//
// The immediate first run matters: a freshly started process — or a feed added a second ago —
// should not sit unpolled for a whole tick, showing an unknown light for no reason.
func Run(ctx context.Context, tick time.Duration, poller Poller) {
	if tick <= 0 {
		tick = DefaultTick
	}

	slog.Info("poller started", "tick", tick)
	pollOnce(ctx, poller)

	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("poller stopped")
			return
		case <-ticker.C:
			pollOnce(ctx, poller)
		}
	}
}

// pollOnce runs the use case and logs what came of it. A failure here is never fatal: the
// next tick tries again.
func pollOnce(ctx context.Context, poller Poller) {
	summary, err := poller.PollDue(ctx)
	if err != nil {
		// A cancelled context is a shutdown, not a fault.
		if ctx.Err() == nil {
			slog.Error("poll run failed", "error", err)
		}
		return
	}
	if summary.Total() == 0 {
		return // nothing was due; saying so every tick would be noise
	}

	slog.Info("polled feeds",
		"updated", summary.Updated,
		"not_modified", summary.NotModified,
		"rate_limited", summary.RateLimited,
		"failed", summary.Failed,
	)
}
