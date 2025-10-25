package pm_logrus

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	"github.com/sirupsen/logrus"
	"github.com/zero-color/pm"
	pm_logging "github.com/zero-color/pm/middleware/logging"
)

// SubscriptionInterceptor returns a subscription interceptor that optionally logs the subscription process.
func SubscriptionInterceptor(logger *logrus.Logger, opt ...Option) pm.SubscriptionInterceptor {
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
			entry := logrus.NewEntry(logger)
			newCtx := newLoggerForProcess(ctx, entry, info, startTime, opts.timestampFormat)

			err := next(ctxlogrus.ToContext(newCtx, entry), m)

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

func newLoggerForProcess(ctx context.Context, entry *logrus.Entry, info *pm.SubscriptionInfo, start time.Time, timestampFormat string) context.Context {
	fields := make(logrus.Fields, 0)
	fields["pubsub.start_time"] = start.Format(timestampFormat)
	if d, ok := ctx.Deadline(); ok {
		fields["pubsub.deadline"] = d.Format(timestampFormat)
	}
	fields["pubsub.subscription_id"] = info.SubscriptionID
	return ctxlogrus.ToContext(ctx, entry.WithFields(fields))
}
