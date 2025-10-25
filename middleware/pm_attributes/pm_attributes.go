package pm_attributes

import (
	"context"

	"cloud.google.com/go/pubsub/v2"
	"github.com/k-yomo/pm"
)

// PublishInterceptor set given attributes to the all publishing messages.
// This interceptor doesn't overwrite if already the same key's attribute is set.
func PublishInterceptor(attrs map[string]string) pm.PublishInterceptor {
	return func(next pm.MessagePublisher) pm.MessagePublisher {
		return func(ctx context.Context, publisher *pubsub.Publisher, m *pubsub.Message) *pubsub.PublishResult {
			for k, v := range attrs {
				if _, ok := m.Attributes[k]; !ok {
					m.Attributes[k] = v
				}
			}
			return next(ctx, publisher, m)
		}
	}
}
