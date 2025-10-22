# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`pm` is a thin Cloud Pub/Sub client wrapper for Google Cloud Platform that provides middleware support for publishing and subscribing to messages. The library implements an interceptor pattern similar to gRPC middleware, allowing developers to compose reusable cross-cutting concerns around message handling.

## Testing

Run the full test suite with emulators:
```bash
make test
```

This command:
1. Starts Docker containers for Pub/Sub emulator (port 8085), Datastore emulator (port 8081), and Redis (port 6379)
2. Runs tests with race detection and coverage
3. Generates both terminal and HTML coverage reports

Run tests without the Makefile:
```bash
docker-compose up -d
PUBSUB_EMULATOR_HOST=localhost:8085 \
  DATASTORE_EMULATOR_HOST=localhost:8081 \
  REDIS_URL=localhost:6379 \
  go test -v -race -coverprofile=coverage.out ./...
```

Run a single test:
```bash
PUBSUB_EMULATOR_HOST=localhost:8085 \
  DATASTORE_EMULATOR_HOST=localhost:8081 \
  REDIS_URL=localhost:6379 \
  go test -v -race -run TestName ./path/to/package
```

## Architecture

### Core Components

**Publisher (`publisher.go`)**
- Wraps `pubsub.Client` to provide middleware-based message publishing
- Applies `PublishInterceptor` middleware in reverse order (like onion layers)
- The actual publish operation happens in the innermost layer

**Subscriber (`subscriber.go`)**
- Manages multiple pull subscriptions with their associated handlers
- Uses `HandleSubscriptionFunc` to register individual handlers or `HandleSubscriptionFuncMap` for batch registration
- `Run()` starts all registered subscriptions in separate goroutines
- Applies `SubscriptionInterceptor` middleware in reverse order around each message handler
- Thread-safe registration using RWMutex

**Batch Handler (`subscriber_batch.go`)**
- Implements `NewBatchMessageHandler` for efficient batch message processing
- Uses `bundler.Bundler` from `google.golang.org/api/support/bundler` to aggregate messages
- Configurable thresholds: delay, count, byte size, and buffered byte limit
- Returns `BatchError` type to handle per-message errors in a batch
- Individual messages are channeled through `bundledMessage` type with error channels for result propagation

### Interceptor Pattern

Both publishing and subscribing use a middleware interceptor pattern:

**PublishInterceptor**
```go
type PublishInterceptor = func(next MessagePublisher) MessagePublisher
```
- Wraps the message publisher function
- Applied in reverse order, so the first interceptor registered is the outermost layer

**SubscriptionInterceptor**
```go
type SubscriptionInterceptor = func(info *SubscriptionInfo, next MessageHandler) MessageHandler
```
- Wraps the message handler function
- Receives `SubscriptionInfo` (TopicID, SubscriptionID) for context
- Applied in reverse order, so the first interceptor registered is the outermost layer

### Middleware Organization

Middleware is organized in `middleware/` directory:
- `pm_attributes/`: Add custom attributes to published messages
- `pm_autoack/`: Automatically ack/nack messages based on handler error
- `pm_effectively_once/`: Message deduplication using Redis, Datastore, or in-memory mutexers
- `pm_recovery/`: Panic recovery with stack trace logging
- `logging/pm_zap/`: Zap logger integration for subscription logging
- `logging/pm_logrus/`: Logrus logger integration for subscription logging

Each middleware package typically exports either `PublishInterceptor()` or `SubscriptionInterceptor()` functions.

### Key Design Patterns

1. **Interceptor Chaining**: Interceptors are applied in reverse order so that the first registered middleware is the outermost layer of the "onion"
2. **Options Pattern**: Both `Publisher` and `Subscriber` use functional options (`PublisherOption`, `SubscriberOption`)
3. **Batch Processing**: Batch handler uses Google's bundler for efficient message aggregation with configurable thresholds
4. **Error Handling**: `BatchError` map type allows per-message error tracking in batch operations
5. **Thread Safety**: Subscriber uses RWMutex for concurrent access to subscription handlers map

## Development Notes

- The library requires Go 1.21 or later
- Tests depend on emulators (Pub/Sub, Datastore) and Redis running locally
- The `pm_effectively_once` middleware requires either Redis, Datastore, or can use in-memory storage for deduplication
- When writing new middleware, follow the interceptor function signature pattern for type safety