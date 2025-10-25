package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"cloud.google.com/go/pubsub/v2"
	"github.com/k-yomo/pm"
	"github.com/k-yomo/pm/middleware/logging/pm_zap"
	"github.com/k-yomo/pm/middleware/pm_attributes"
	"github.com/k-yomo/pm/middleware/pm_autoack"
	"github.com/k-yomo/pm/middleware/pm_recovery"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, "pm-example")
	if err != nil {
		logger.Fatal("initialize pubsub client failed", zap.Error(err))
	}
	defer pubsubClient.Close()

	pubsubPublisher := pm.NewPublisher(
		pubsubClient,
		pm.WithPublishInterceptor(
			pm_attributes.PublishInterceptor(map[string]string{"key": "value"}),
		),
	)

	pubsubSubscriber := pm.NewSubscriber(
		pubsubClient,
		pm.WithSubscriptionInterceptor(
			pm_zap.SubscriptionInterceptor(logger),
			pm_autoack.SubscriptionInterceptor(),
			pm_recovery.SubscriptionInterceptor(pm_recovery.WithDebugRecoveryHandler()),
		),
	)
	defer pubsubSubscriber.Close()

	sub := pubsubClient.Subscriber("example-topic-sub")
	batchSub := pubsubClient.Subscriber("example-topic-batch-sub")
	err = pubsubSubscriber.HandleSubscriptionFuncMap("example-topic", map[*pubsub.Subscriber]pm.MessageHandler{
		sub: exampleSubscriptionHandler,
		batchSub: pm.NewBatchMessageHandler(exampleSubscriptionBatchHandler, pm.BatchMessageHandlerConfig{
			DelayThreshold:    100,
			CountThreshold:    1000,
			ByteThreshold:     1e6,
			BufferedByteLimit: 1e8,
		}),
	})
	if err != nil {
		logger.Fatal("register subscription failed", zap.Error(err))
	}

	pubsubSubscriber.Run(ctx)
	defer pubsubSubscriber.Close()

	pubsubPublisher.Publish(
		ctx,
		pubsubClient.Publisher("example-topic"),
		&pubsub.Message{
			Data: []byte("test"),
		},
	)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func exampleSubscriptionHandler(ctx context.Context, m *pubsub.Message) error {
	dataStr := string(m.Data)
	if dataStr == "panic" {
		panic("panic")
	}

	if dataStr == "error" {
		return errors.New("error")
	}

	fmt.Println(dataStr)
	return nil
}

func exampleSubscriptionBatchHandler(messages []*pubsub.Message) error {
	batchErr := make(pm.BatchError)
	for _, m := range messages {
		dataStr := string(m.Data)
		if dataStr == "error" {
			batchErr[m.ID] = errors.New("error")
		} else {
			fmt.Println(dataStr)
		}
	}

	return batchErr
}
