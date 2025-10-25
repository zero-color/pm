package pm_logrus

import (
	"context"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	"github.com/sirupsen/logrus"
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
	logger := ctxlogrus.Extract(ctx).WithFields(map[string]interface{}{
		"pubsub.time_ms": pm_logging.DurationToMilliseconds(duration),
	})
	if err != nil {
		logger.WithField(logrus.ErrorKey, err).Error(msg)
	} else {
		logger.Info(msg)
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
