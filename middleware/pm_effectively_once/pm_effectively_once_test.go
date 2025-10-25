package pm_effectively_once

import (
	"context"
	"sync"
	"testing"

	"cloud.google.com/go/pubsub/v2"
	"github.com/zero-color/pm"
)

type testMutexer struct {
	sync.Mutex
	passedDeduplicateKey string
}

func (d *testMutexer) RunInTx(_ context.Context, deduplicateKey string, f func() error) error {
	d.passedDeduplicateKey = deduplicateKey
	return f()
}

func TestSubscriptionInterceptor(t *testing.T) {
	next := func(ctx context.Context, m *pubsub.Message) error {
		return nil
	}

	t.Run("when de-duplicate key exists in the attributes, RunInTx is called with the key", func(t *testing.T) {
		mutexer := testMutexer{}
		interceptor := SubscriptionInterceptor(&mutexer)
		_ = interceptor(&pm.SubscriptionInfo{}, next)(context.Background(), &pubsub.Message{ID: "messageID", Attributes: map[string]string{DefaultDeduplicateKey: "test"}})
		if got := mutexer.passedDeduplicateKey; got != "test" {
			t.Errorf("TestSubscriptionInterceptor(): got: %v, want: %v", got, "test")
		}
	})
	t.Run("when de-duplicate key does exist in the attributes, RunInTx is called with message id", func(t *testing.T) {
		mutexer := testMutexer{}
		interceptor := SubscriptionInterceptor(&mutexer)
		_ = interceptor(&pm.SubscriptionInfo{}, next)(context.Background(), &pubsub.Message{ID: "messageID", Attributes: map[string]string{}})
		if got := mutexer.passedDeduplicateKey; got != "messageID" {
			t.Errorf("TestSubscriptionInterceptor(): got: %v, want: %v", got, "messageID")
		}
	})
}
