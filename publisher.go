package pm

import (
	"context"

	"cloud.google.com/go/pubsub/v2"
)

// MessagePublisher defines the message publisher invoked by PublishInterceptor to complete the normal
// message publishment.
type MessagePublisher = func(ctx context.Context, topic *pubsub.Publisher, m *pubsub.Message) *pubsub.PublishResult

// PublishInterceptor provides a hook to intercept the execution of a publishment.
type PublishInterceptor = func(next MessagePublisher) MessagePublisher

// Publisher represents a wrapper of Pub/Sub client focusing on publishment.
type Publisher struct {
	opts *publisherOptions
	*pubsub.Client
}

// NewPublisher initializes new Publisher.
func NewPublisher(pubsubClient *pubsub.Client, opt ...PublisherOption) *Publisher {
	opts := publisherOptions{}
	for _, o := range opt {
		o.apply(&opts)
	}
	return &Publisher{
		&opts,
		pubsubClient,
	}
}

// Publish publishes Pub/Sub message with applying middlewares
func (p *Publisher) Publish(ctx context.Context, publisher *pubsub.Publisher, m *pubsub.Message) *pubsub.PublishResult {
	last := publish
	for i := len(p.opts.publishInterceptors) - 1; i >= 0; i-- {
		last = p.opts.publishInterceptors[i](last)
	}
	return last(ctx, publisher, m)
}

func publish(ctx context.Context, publisher *pubsub.Publisher, m *pubsub.Message) *pubsub.PublishResult {
	return publisher.Publish(ctx, m)
}
