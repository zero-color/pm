package pm_logrus

import (
	"context"
	"errors"
	"testing"

	"cloud.google.com/go/pubsub/v2"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/zero-color/pm"
)

func TestSubscriptionInterceptor(t *testing.T) {
	t.Parallel()

	testSubInfo := &pm.SubscriptionInfo{
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

			logger, hook := test.NewNullLogger()
			interceptor := SubscriptionInterceptor(logger)
			callHandler(interceptor(testSubInfo, successMessageHandler))

			if got := len(hook.Entries); got != 1 {
				t.Fatalf("Only 1 log is expected to be emitted, got: %v, want: %v", got, 1)
			}
			entry := hook.Entries[0]
			if entry.Level != logrus.InfoLevel {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", entry.Level, logrus.InfoLevel)
			}
			wantMessage := "finished processing message 'message-id'"
			if entry.Message != wantMessage {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", entry.Message, wantMessage)
			}
		})

		t.Run("Emit error log when processing is successful", func(t *testing.T) {
			t.Parallel()

			logger, hook := test.NewNullLogger()
			interceptor := SubscriptionInterceptor(logger)
			callHandler(interceptor(testSubInfo, failureMessageHandler))

			if got := len(hook.Entries); got != 1 {
				t.Fatalf("Only 1 log is expected to be emitted, got: %v, want: %v", got, 1)
			}
			entry := hook.Entries[0]
			if entry.Level != logrus.ErrorLevel {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", entry.Level, logrus.ErrorLevel)
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

			logger, hook := test.NewNullLogger()
			interceptor := SubscriptionInterceptor(logger, WithLogDecider(func(info *pm.SubscriptionInfo, err error) bool {
				return false
			}))
			callHandler(interceptor(testSubInfo, successMessageHandler))

			if len(hook.Entries) != 0 {
				t.Errorf("log is not expected to be emitted")
			}
		})
	})
}
