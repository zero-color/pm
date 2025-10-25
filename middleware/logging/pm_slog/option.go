package pm_slog

import (
	"context"
	"log/slog"
	"time"

	pm_logging "github.com/zero-color/pm/middleware/logging"
)

type options struct {
	shouldLog       pm_logging.LogDecider
	messageProducer MessageProducer
	timestampFormat string
}

// MessageProducer produces a user defined log message
type MessageProducer func(ctx context.Context, msg string, err error, duration time.Duration)

// DefaultMessageProducer writes the default message
func DefaultMessageProducer(ctx context.Context, msg string, err error, duration time.Duration) {
	logger := Extract(ctx)
	durationAttr := slog.Float64("pubsub.time_ms", float64(pm_logging.DurationToMilliseconds(duration)))
	if err != nil {
		logger.ErrorContext(ctx, msg, slog.Any("error", err), durationAttr)
	} else {
		logger.InfoContext(ctx, msg, durationAttr)
	}
}

// Option is a option to change configuration.
type Option interface {
	apply(*options)
}

type OptionFunc struct {
	f func(*options)
}

func (s *OptionFunc) apply(so *options) {
	s.f(so)
}

func newOptionFunc(f func(*options)) *OptionFunc {
	return &OptionFunc{
		f: f,
	}
}

// WithLogDecider customizes the function for deciding if the pm interceptor should log.
func WithLogDecider(f pm_logging.LogDecider) Option {
	return newOptionFunc(func(o *options) {
		o.shouldLog = f
	})
}

// WithMessageProducer customizes the function for logging.
func WithMessageProducer(f MessageProducer) Option {
	return newOptionFunc(func(o *options) {
		o.messageProducer = f
	})
}
