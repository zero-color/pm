package pm_recovery

import (
	"context"
	"testing"

	"cloud.google.com/go/pubsub/v2"
	"github.com/k-yomo/pm"
)

func TestSubscriptionInterceptor(t *testing.T) {
	t.Parallel()

	next := func(ctx context.Context, m *pubsub.Message) error {
		panic("panic")
	}

	t.Run("recovers with default recovery handler", func(t *testing.T) {
		t.Parallel()
		interceptor := SubscriptionInterceptor()
		_ = interceptor(&pm.SubscriptionInfo{}, next)(context.Background(), &pubsub.Message{})
	})

	t.Run("recovers with custom recovery handler", func(t *testing.T) {
		t.Parallel()

		var called bool
		opts := []Option{WithRecoveryHandler(func(ctx context.Context, p interface{}) {
			called = true
		})}
		interceptor := SubscriptionInterceptor(opts...)
		_ = interceptor(&pm.SubscriptionInfo{}, next)(context.Background(), &pubsub.Message{})
		if !called {
			t.Error("The custom recovery handler is not called")
		}
	})

	t.Run("recovers with debug recovery handler", func(t *testing.T) {
		t.Parallel()

		opts := []Option{WithDebugRecoveryHandler()}
		interceptor := SubscriptionInterceptor(opts...)
		_ = interceptor(&pm.SubscriptionInfo{}, next)(context.Background(), &pubsub.Message{})
	})
}
