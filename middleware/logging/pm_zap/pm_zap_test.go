package pm_zap

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/pubsub/v2"
	"github.com/k-yomo/pm"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
)

func TestSubscriptionInterceptor(t *testing.T) {
	t.Parallel()

	testSubInfo := &pm.SubscriptionInfo{
		TopicID:        "test-topic",
		SubscriptionID: "test-sub",
	}

	successMessageHandler := func(ctx context.Context, m *pubsub.Message) error {
		return nil
	}
	failureMessageHandler := func(ctx context.Context, m *pubsub.Message) error {
		return errors.New("error")
	}

	callHandler := func(f pm.MessageHandler) {
		_ = f(context.Background(), &pubsub.Message{ID: "message-id"})
	}

	t.Run("with default options", func(t *testing.T) {
		t.Run("emit info log when processing is successful", func(t *testing.T) {
			t.Parallel()

			core, obs := zapobserver.New(zap.InfoLevel)
			logger := zap.New(core)

			interceptor := SubscriptionInterceptor(logger)
			callHandler(interceptor(testSubInfo, successMessageHandler))

			if got := obs.Len(); got != 1 {
				t.Fatalf("Only 1 log is expected to be emitted, got: %v, want: %v", got, 1)
			}
			entry := obs.All()[0]
			if got := entry.Level; got != zap.InfoLevel {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", got, zap.InfoLevel)
			}
			wantMessage := "finished processing message 'message-id'"
			if entry.Message != wantMessage {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", entry.Message, wantMessage)
			}
		})

		t.Run("Emit error log when processing is successful", func(t *testing.T) {
			t.Parallel()

			core, obs := zapobserver.New(zap.ErrorLevel)
			logger := zap.New(core)

			interceptor := SubscriptionInterceptor(logger)
			callHandler(interceptor(testSubInfo, failureMessageHandler))

			if got := obs.Len(); got != 1 {
				t.Fatalf("Only 1 log is expected to be emitted, got: %v, want: %v", got, 1)
			}
			entry := obs.All()[0]
			if got := entry.Level; got != zap.ErrorLevel {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", got, zap.ErrorLevel)
			}
			wantMessage := "finished processing message 'message-id'"
			if entry.Message != wantMessage {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", entry.Message, wantMessage)
			}
		})
	})

	t.Run("with custom options", func(t *testing.T) {
		t.Run("custom options are applied", func(t *testing.T) {
			t.Parallel()

			core, obs := zapobserver.New(zap.DebugLevel)
			logger := zap.New(core)

			interceptor := SubscriptionInterceptor(logger, WithLogDecider(func(info *pm.SubscriptionInfo, err error) bool {
				return false
			}))
			callHandler(interceptor(testSubInfo, successMessageHandler))

			if obs.Len() != 0 {
				t.Errorf("log is not expected to be emitted")
			}
		})
	})
}
