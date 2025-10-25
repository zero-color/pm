package pm_slog

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"cloud.google.com/go/pubsub/v2"
	"github.com/k-yomo/pm"
)

// testLogEntry represents a captured log entry
type testLogEntry struct {
	level   slog.Level
	message string
	attrs   map[string]any
}

// testHandler is a custom slog.Handler that captures log entries for testing
type testHandler struct {
	mu      sync.Mutex
	entries *[]testLogEntry
	level   slog.Level
}

func newTestHandler(level slog.Level) *testHandler {
	entries := make([]testLogEntry, 0)
	return &testHandler{
		entries: &entries,
		level:   level,
	}
}

func (h *testHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *testHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	attrs := make(map[string]any)
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	*h.entries = append(*h.entries, testLogEntry{
		level:   r.Level,
		message: r.Message,
		attrs:   attrs,
	})
	return nil
}

func (h *testHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For testing purposes, we create a new handler that shares the same entries pointer
	newHandler := &testHandler{
		entries: h.entries,
		level:   h.level,
	}
	return newHandler
}

func (h *testHandler) WithGroup(name string) slog.Handler {
	return h
}

func (h *testHandler) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(*h.entries)
}

func (h *testHandler) All() []testLogEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]testLogEntry, len(*h.entries))
	copy(result, *h.entries)
	return result
}

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

			handler := newTestHandler(slog.LevelInfo)
			logger := slog.New(handler)

			interceptor := SubscriptionInterceptor(logger)
			callHandler(interceptor(testSubInfo, successMessageHandler))

			if got := handler.Len(); got != 1 {
				t.Fatalf("Only 1 log is expected to be emitted, got: %v, want: %v", got, 1)
			}
			entry := handler.All()[0]
			if got := entry.level; got != slog.LevelInfo {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", got, slog.LevelInfo)
			}
			wantMessage := "finished processing message 'message-id'"
			if entry.message != wantMessage {
				t.Errorf("INFO log is expected to be emitted, got: %v, want: %v", entry.message, wantMessage)
			}
		})

		t.Run("Emit error log when processing fails", func(t *testing.T) {
			t.Parallel()

			handler := newTestHandler(slog.LevelError)
			logger := slog.New(handler)

			interceptor := SubscriptionInterceptor(logger)
			callHandler(interceptor(testSubInfo, failureMessageHandler))

			if got := handler.Len(); got != 1 {
				t.Fatalf("Only 1 log is expected to be emitted, got: %v, want: %v", got, 1)
			}
			entry := handler.All()[0]
			if got := entry.level; got != slog.LevelError {
				t.Errorf("ERROR log is expected to be emitted, got: %v, want: %v", got, slog.LevelError)
			}
			wantMessage := "finished processing message 'message-id'"
			if entry.message != wantMessage {
				t.Errorf("ERROR log is expected to be emitted, got: %v, want: %v", entry.message, wantMessage)
			}
		})
	})

	t.Run("with custom options", func(t *testing.T) {
		t.Run("custom options are applied", func(t *testing.T) {
			t.Parallel()

			handler := newTestHandler(slog.LevelDebug)
			logger := slog.New(handler)

			interceptor := SubscriptionInterceptor(logger, WithLogDecider(func(info *pm.SubscriptionInfo, err error) bool {
				return false
			}))
			callHandler(interceptor(testSubInfo, successMessageHandler))

			if handler.Len() != 0 {
				t.Errorf("log is not expected to be emitted")
			}
		})
	})
}
