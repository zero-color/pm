package pm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub/v2"
	pb "cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/support/bundler"
	"google.golang.org/protobuf/proto"
)

// BatchError is used to handle error for each message
// The key is message id
type BatchError map[string]error

func (b BatchError) Error() string {
	errStrings := make([]string, 0, len(b))
	for messageID, err := range b {
		errStrings = append(errStrings, fmt.Sprintf("%s for message '%s'", err.Error(), messageID))
	}
	return strings.Join(errStrings, ", ")
}

// MessageBatchHandler defines the batch message handler
// By default, when non-nil error is returned, all messages are processed as error in MessageHandler
// To handle error for each message, use BatchError
type MessageBatchHandler func(messages []*pubsub.Message) error

type BatchMessageHandlerConfig struct {
	// Process a non-empty batch after this delay has passed.
	// Defaults to DefaultMessageBatchHandlerConfig.DelayThreshold.
	DelayThreshold time.Duration

	// Process a batch when it has this many messages.
	// Defaults to DefaultMessageBatchHandlerConfig.CountThreshold.
	CountThreshold int

	// Process a batch when its size in bytes reaches this value.
	// Defaults to DefaultMessageBatchHandlerConfig.ByteThreshold.
	ByteThreshold int

	// The number of goroutines.
	// Defaults to DefaultMessageBatchHandlerConfig.NumGoroutines.
	NumGoroutines int

	// Defaults to DefaultMessageBatchHandlerConfig.BufferedByteLimit.
	BufferedByteLimit int
}

var DefaultMessageBatchHandlerConfig = &BatchMessageHandlerConfig{
	DelayThreshold: 10 * time.Millisecond,
	CountThreshold: 100,
	ByteThreshold:  1e6, // 1MB
	NumGoroutines:  10,
	// By default, limit the bundler to 10 times the max msg size. The number 10 is
	// chosen as a reasonable amount of messages in the worst case whilst still
	// capping the number to a low enough value to not OOM users.
	BufferedByteLimit: 10 * pubsub.MaxPublishRequestBytes,
}

type messageBatchHandleScheduler struct {
	mu      sync.RWMutex
	bundler *bundler.Bundler
}

// NewBatchMessageHandler initializes MessageHandler for batch message processing with config
func NewBatchMessageHandler(handler MessageBatchHandler, config BatchMessageHandlerConfig) MessageHandler {
	return newMessageBatchHandler(handler, config)
}

func newMessageBatchHandler(handler MessageBatchHandler, config BatchMessageHandlerConfig) MessageHandler {
	batchScheduler := newMessageBatchHandleScheduler(handler, config)
	return func(ctx context.Context, msg *pubsub.Message) error {
		errCh := make(chan error)
		bm := bundledMessage{msg: msg, err: errCh}
		if err := batchScheduler.add(&bm); err != nil {
			return err
		}

		err := <-errCh
		return err
	}
}

func newMessageBatchHandleScheduler(handler MessageBatchHandler, config BatchMessageHandlerConfig) *messageBatchHandleScheduler {
	if config.DelayThreshold == 0 {
		config.DelayThreshold = DefaultMessageBatchHandlerConfig.DelayThreshold
	}
	if config.CountThreshold == 0 {
		config.CountThreshold = DefaultMessageBatchHandlerConfig.CountThreshold
	}
	if config.ByteThreshold == 0 {
		config.ByteThreshold = DefaultMessageBatchHandlerConfig.ByteThreshold
	}
	if config.NumGoroutines == 0 {
		config.NumGoroutines = DefaultMessageBatchHandlerConfig.NumGoroutines
	}
	if config.BufferedByteLimit == 0 {
		config.BufferedByteLimit = DefaultMessageBatchHandlerConfig.BufferedByteLimit
	}

	return &messageBatchHandleScheduler{
		bundler: newBundler(handler, config),
	}
}

type bundledMessage struct {
	msg *pubsub.Message
	err chan<- error
}

func (m *messageBatchHandleScheduler) add(bm *bundledMessage) error {
	msgSize := proto.Size(&pb.PubsubMessage{
		Data:        bm.msg.Data,
		Attributes:  bm.msg.Attributes,
		MessageId:   bm.msg.ID,
		OrderingKey: bm.msg.OrderingKey,
	})
	return m.bundler.Add(bm, msgSize)
}

func newBundler(handler MessageBatchHandler, config BatchMessageHandlerConfig) *bundler.Bundler {
	b := bundler.NewBundler(&bundledMessage{}, newBundleHandler(handler))
	b.HandlerLimit = config.NumGoroutines
	b.DelayThreshold = config.DelayThreshold
	b.BundleCountThreshold = config.CountThreshold
	b.BundleByteThreshold = config.ByteThreshold
	b.BufferedByteLimit = config.BufferedByteLimit

	return b
}

func newBundleHandler(handler MessageBatchHandler) func(bundle interface{}) {
	return func(bundle interface{}) {
		bundledMessages := bundle.([]*bundledMessage)

		messages := make([]*pubsub.Message, 0, len(bundledMessages))
		for _, bm := range bundledMessages {
			messages = append(messages, bm.msg)
		}

		err := handler(messages)
		var batchErr BatchError
		if errors.As(err, &batchErr) {
			for _, bm := range bundledMessages {
				bm.err <- batchErr[bm.msg.ID]
			}
		} else {
			// For non-BatchError (including nil), send the same error to all messages
			for _, bm := range bundledMessages {
				bm.err <- err
			}
		}
	}
}
