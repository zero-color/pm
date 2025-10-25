// Package pm_slog provides a slog-based logging interceptor for pm subscriptions.
//
// Example usage:
//
//	import (
//		"log/slog"
//		"github.com/zero-color/pm"
//		"github.com/zero-color/pm/middleware/logging/pm_slog"
//	)
//
//	logger := slog.Default()
//	subscriber := pm.NewSubscriber(
//		client,
//		pm.WithSubscriptionInterceptors(
//			pm_slog.SubscriptionInterceptor(logger),
//		),
//	)
package pm_slog

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/zero-color/pm"
	pm_logging "github.com/zero-color/pm/middleware/logging"
)

type ctxLoggerMarker struct{}

var (
	ctxLoggerKey = &ctxLoggerMarker{}
)

// Extract returns the slog.Logger from the context.
// If no logger is found, it returns the default logger.
func Extract(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxLoggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// ToContext adds the slog.Logger to the context.
func ToContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey, logger)
}

// SubscriptionInterceptor returns a subscription interceptor that optionally logs the subscription process.
func SubscriptionInterceptor(logger *slog.Logger, opt ...Option) pm.SubscriptionInterceptor {
	opts := &options{
		shouldLog:       pm_logging.DefaultLogDecider,
		messageProducer: DefaultMessageProducer,
		timestampFormat: time.RFC3339,
	}
	for _, o := range opt {
		o.apply(opts)
	}

	return func(info *pm.SubscriptionInfo, next pm.MessageHandler) pm.MessageHandler {
		return func(ctx context.Context, m *pubsub.Message) error {
			startTime := time.Now()
			newCtx := newLoggerForProcess(ctx, logger, info, startTime, opts.timestampFormat)

			err := next(newCtx, m)

			if opts.shouldLog(info, err) {
				opts.messageProducer(
					newCtx, fmt.Sprintf("finished processing message '%s'", m.ID),
					err,
					time.Since(startTime),
				)
			}
			return err
		}
	}
}

func newLoggerForProcess(ctx context.Context, logger *slog.Logger, info *pm.SubscriptionInfo, start time.Time, timestampFormat string) context.Context {
	args := []any{
		"pubsub.start_time", start.Format(timestampFormat),
	}
	if d, ok := ctx.Deadline(); ok {
		args = append(args, "pubsub.deadline", d.Format(timestampFormat))
	}
	args = append(args,
		"pubsub.topic_id", info.TopicID,
		"pubsub.subscription_id", info.SubscriptionID,
	)
	return ToContext(ctx, logger.With(args...))
}
