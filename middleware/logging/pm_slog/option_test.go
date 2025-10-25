package pm_slog

import (
	"context"
	"testing"
	"time"

	"github.com/zero-color/pm"
)

func TestWithLogDecider(t *testing.T) {
	t.Parallel()

	opts := options{}
	isDecided := false
	customDecider := func(info *pm.SubscriptionInfo, err error) bool {
		isDecided = true
		return true
	}
	WithLogDecider(customDecider).apply(&opts)
	opts.shouldLog(nil, nil)
	if !isDecided {
		t.Errorf("WithLogDecider() is expected to set custom decider, but it was not set")
	}
}

func TestWithMessageProducer(t *testing.T) {
	t.Parallel()

	opts := options{}

	isProduced := false
	customMessageProducer := func(ctx context.Context, msg string, err error, duration time.Duration) {
		isProduced = true
	}
	WithMessageProducer(customMessageProducer).apply(&opts)
	opts.messageProducer(nil, "", nil, 0)
	if !isProduced {
		t.Errorf("WithMessageProducer() is expected to set custom message producer, but it was not set")
	}
}
