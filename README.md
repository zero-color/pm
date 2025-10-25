# pm

![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)
![Test Workflow](https://github.com/k-yomo/pm/workflows/Test/badge.svg)
[![codecov](https://codecov.io/gh/k-yomo/pm/branch/main/graph/badge.svg)](https://codecov.io/gh/k-yomo/pm)
[![Go Report Card](https://goreportcard.com/badge/k-yomo/pm)](https://goreportcard.com/report/k-yomo/pm)

pm is a thin Cloud Pub/Sub client wrapper which lets you manage publishing / subscribing with pluggable middleware.

## Installation

```sh
go get -u github.com/k-yomo/pm
```

## Example

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
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

	sub := pubsubClient.Subscription("example-topic-sub")
	batchSub := pubsubClient.Subscription("example-topic-batch-sub")
	err = pubsubSubscriber.HandleSubscriptionFuncMap(map[*pubsub.Subscription]pm.MessageHandler{
		sub: exampleSubscriptionHandler,
		batchSub: pm.NewBatchMessageHandler(exampleSubscriptionBatchHandler, pm.BatchMessageHandlerConfig{
			DelayThreshold:    100 * time.Millisecond,
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
		pubsubPublisher.Topic("example-topic"),
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
```

## Middlewares

### Core Middleware

pm comes equipped with an optional middleware packages named `pm_*`.

#### Publish interceptor

| interceptor                                                                                                | description                                                              |
|------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------|
| [Attributes](https://pkg.go.dev/github.com/k-yomo/pm/middleware/pm_attributes#PublishInterceptor)          | Set custom attributes to all outgoing messages when publish              |

#### Subscription interceptor

| interceptor                                                                                                        | description                                                              |
|--------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------|
| [Auto Ack](https://pkg.go.dev/github.com/k-yomo/pm/middleware/pm_autoack#SubscriptionInterceptor)                 | Ack automatically depending on if error is returned when subscribe       |
| [Effectively Once](https://pkg.go.dev/github.com/k-yomo/pm/middleware/pm_effectively_once#SubscriptionInterceptor)| De-duplicate messages with the same de-duplicate key                     |
| [Logging - Zap](https://pkg.go.dev/github.com/k-yomo/pm/middleware/logging/pm_zap#SubscriptionInterceptor)        | Emit an informative zap log when subscription processing finish          |
| [Logging - Logrus](https://pkg.go.dev/github.com/k-yomo/pm/middleware/logging/pm_logrus#SubscriptionInterceptor) | Emit an informative logrus log when subscription processing finish       |
| [Logging - Slog](https://pkg.go.dev/github.com/k-yomo/pm/middleware/logging/pm_slog#SubscriptionInterceptor)     | Emit an informative slog log when subscription processing finish         |
| [Recovery](https://pkg.go.dev/github.com/k-yomo/pm/middleware#SubscriptionInterceptor)                | Gracefully recover from panics and prints the stack trace when subscribe |

#### Custom Middleware

pm middleware is just wrapping publishing / subscribing process which means you can define your custom middleware as well.
- publish interceptor
```go
func MyPublishInterceptor(attrs map[string]string) pm.PublishInterceptor {
	return func (next pm.MessagePublisher) pm.MessagePublisher {
		return func (ctx context.Context, topic *pubsub.Topic, m *pubsub.Message) *pubsub.PublishResult {
			// do something before publishing 
			result := next(ctx, topic, m)
			// do something after publishing 
			return result
		}
	}
}
```

- subscription interceptor
```go
func MySubscriptionInterceptor() pm.SubscriptionInterceptor {
	return func(_ *pm.SubscriptionInfo, next pm.MessageHandler) pm.MessageHandler {
		return func(ctx context.Context, m *pubsub.Message) error {
			// do something before subscribing 
			err := next(ctx, m) 
			// do something after subscribing 
			return err
		}
	}
}
```
