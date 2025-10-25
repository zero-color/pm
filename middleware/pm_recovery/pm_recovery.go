package pm_recovery

import (
	"context"

	"cloud.google.com/go/pubsub/v2"
	"github.com/k-yomo/pm"
)

// RecoveryHandlerFunc is a function that recovers from the panic `p`.
// The context can be used to extract metadata and context values.
type RecoveryHandlerFunc func(ctx context.Context, p interface{})

// SubscriptionInterceptor recover panic.
func SubscriptionInterceptor(opt ...Option) pm.SubscriptionInterceptor {
	opts := options{
		recoveryHandlerFunc: defaultRecoveryHandler,
	}
	for _, o := range opt {
		o(&opts)
	}
	return func(_ *pm.SubscriptionInfo, next pm.MessageHandler) pm.MessageHandler {
		return func(ctx context.Context, m *pubsub.Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					opts.recoveryHandlerFunc(ctx, r)
				}
			}()
			return next(ctx, m)
		}
	}
}
