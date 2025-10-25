package pm_effectively_once

import (
	"context"

	"cloud.google.com/go/pubsub/v2"
	"github.com/zero-color/pm"
)

type Mutexer interface {
	RunInTx(ctx context.Context, deduplicateKey string, f func() error) error
}

// SubscriptionInterceptor process only the first event and discards the others with the same de-duplicate key.
// To make this interceptor work, you need to set the de-duplicate key in the attributes when publishing message like below.
// If the key is not set, messageID will be used as the de-duplicate key.
//
// // publisher
// msg := pubsub.Message{Data: []byte("something"), Attributes: map[string]string{pm_effectively_once.DeduplicateKey: "unique-key"}}
// pubsub.Topic{}.Publish(ctx, msg)
//
// // subscriber
// pubsubSubscriber := pm.NewSubscriber(
//
//		pubsubClient,
//		pm.WithSubscriptionInterceptor(
//			pm_effectively_once.SubscriptionInterceptor(pm_effectively_once.NewRedisMutexer(redisClient)),
//		),
//	)
func SubscriptionInterceptor(mutexer Mutexer, opt ...Option) pm.SubscriptionInterceptor {
	opts := options{
		deduplicateKey: DefaultDeduplicateKey,
	}
	for _, o := range opt {
		o(&opts)
	}
	return func(_ *pm.SubscriptionInfo, next pm.MessageHandler) pm.MessageHandler {
		return func(ctx context.Context, m *pubsub.Message) error {
			deduplicateKey := m.ID
			// when we find the deduplicateKey in the attributes, we prioritise it.
			if keyInAttrs, ok := m.Attributes[DefaultDeduplicateKey]; ok {
				deduplicateKey = keyInAttrs
			}
			return mutexer.RunInTx(ctx, deduplicateKey, func() error {
				return next(ctx, m)
			})
		}
	}
}
