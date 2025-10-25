package pm

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/support/bundler"
)

func TestBatchError_Error(t *testing.T) {
	tests := []struct {
		name string
		b    BatchError
		want string
	}{
		{
			name: "returns error string",
			b: map[string]error{
				"1": errors.New("error 1"),
			},
			want: "error 1 for message '1'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_MessageBatchHandler(t *testing.T) {
	t.Parallel()

	t.Run("messages will be processed in batch when reached the count threshold", func(t *testing.T) {
		t.Parallel()

		batchConfig := BatchMessageHandlerConfig{
			DelayThreshold:    100 * time.Millisecond,
			CountThreshold:    2,
			ByteThreshold:     1e5,
			NumGoroutines:     1,
			BufferedByteLimit: 1e5,
		}
		msgHandler := NewBatchMessageHandler(func(messages []*pubsub.Message) error {
			if len(messages) != 2 {
				t.Errorf("got message count = %v, want %v", len(messages), 2)
			}
			return nil
		}, batchConfig)

		eg := errgroup.Group{}
		eg.Go(func() error {
			return msgHandler(context.Background(), &pubsub.Message{})
		})
		eg.Go(func() error {
			return msgHandler(context.Background(), &pubsub.Message{})
		})
		if err := eg.Wait(); err != nil {
			t.Errorf("Error() = %v, want %v", err, nil)
		}
	})

	t.Run("all message handlers receive error", func(t *testing.T) {
		t.Parallel()

		batchConfig := BatchMessageHandlerConfig{
			DelayThreshold:    100 * time.Millisecond,
			CountThreshold:    2,
			ByteThreshold:     DefaultMessageBatchHandlerConfig.ByteThreshold,
			NumGoroutines:     1,
			BufferedByteLimit: DefaultMessageBatchHandlerConfig.BufferedByteLimit,
		}
		expectedError := errors.New("error")
		msgHandler := NewBatchMessageHandler(func(messages []*pubsub.Message) error {
			return errors.New("error")
		}, batchConfig)

		eg := errgroup.Group{}
		eg.Go(func() error {
			if err := msgHandler(context.Background(), &pubsub.Message{}); err == nil {
				t.Errorf("Error() = %v, want %v", err, expectedError)
			}
			return nil
		})
		eg.Go(func() error {
			if err := msgHandler(context.Background(), &pubsub.Message{}); err == nil {
				t.Errorf("Error() = %v, want %v", err, expectedError)
			}
			return nil
		})
		if err := eg.Wait(); err != nil {
			t.Errorf("Error() = %v, want %v", err, nil)
		}
	})

	t.Run("specific message handlers receive error", func(t *testing.T) {
		t.Parallel()

		batchConfig := BatchMessageHandlerConfig{
			DelayThreshold:    100 * time.Millisecond,
			CountThreshold:    2,
			ByteThreshold:     DefaultMessageBatchHandlerConfig.ByteThreshold,
			NumGoroutines:     1,
			BufferedByteLimit: DefaultMessageBatchHandlerConfig.BufferedByteLimit,
		}
		wantErr := errors.New("error")
		msgHandler := NewBatchMessageHandler(func(messages []*pubsub.Message) error {
			return BatchError{"1": wantErr}
		}, batchConfig)

		eg := errgroup.Group{}
		eg.Go(func() error {
			if err := msgHandler(context.Background(), &pubsub.Message{ID: "1"}); err != wantErr {
				t.Errorf("Error() = %v, want %v", err, wantErr)
			}
			return nil
		})
		eg.Go(func() error {
			if err := msgHandler(context.Background(), &pubsub.Message{ID: "2"}); err != nil {
				t.Errorf("Error() = %v, want %v", err, nil)
			}
			return nil
		})
		if err := eg.Wait(); err != nil {
			t.Errorf("Error() = %v, want %v", err, nil)
		}
	})
}

func Test_newMessageBatchHandleScheduler(t *testing.T) {
	t.Parallel()

	testHanlder := func(messages []*pubsub.Message) error {
		return nil
	}

	type args struct {
		handler MessageBatchHandler
		config  BatchMessageHandlerConfig
	}
	tests := []struct {
		name        string
		args        args
		wantBundler *bundler.Bundler
	}{
		{
			name: "initialize messageBatchHandleScheduler with default config",
			args: args{handler: testHanlder, config: BatchMessageHandlerConfig{}},
			wantBundler: &bundler.Bundler{
				DelayThreshold:       DefaultMessageBatchHandlerConfig.DelayThreshold,
				BundleCountThreshold: DefaultMessageBatchHandlerConfig.CountThreshold,
				BundleByteThreshold:  DefaultMessageBatchHandlerConfig.ByteThreshold,
				BundleByteLimit:      0,
				BufferedByteLimit:    DefaultMessageBatchHandlerConfig.BufferedByteLimit,
				HandlerLimit:         10,
			},
		},
		{
			name: "initialize messageBatchHandleScheduler with custom config",
			args: args{
				handler: testHanlder,
				config: BatchMessageHandlerConfig{
					DelayThreshold:    100 * time.Millisecond,
					CountThreshold:    10,
					ByteThreshold:     1e4,
					NumGoroutines:     5,
					BufferedByteLimit: 1e8,
				},
			},
			wantBundler: &bundler.Bundler{
				DelayThreshold:       100 * time.Millisecond,
				BundleCountThreshold: 10,
				BundleByteThreshold:  1e4,
				BundleByteLimit:      0,
				BufferedByteLimit:    1e8,
				HandlerLimit:         5,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newMessageBatchHandleScheduler(tt.args.handler, tt.args.config)
			if diff := cmp.Diff(got.bundler, tt.wantBundler, cmpopts.IgnoreUnexported(*got.bundler)); diff != "" {
				t.Errorf("newMessageBatchHandleScheduler() = (-want +got):\n%s", diff)
			}
		})
	}
}
