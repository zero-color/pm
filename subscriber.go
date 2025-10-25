package pm

import (
	"context"
	"fmt"
	"log"
	"sync"

	"cloud.google.com/go/pubsub/v2"
	"golang.org/x/sync/errgroup"
)

// Subscriber represents a wrapper of Pub/Sub client mainly focusing on pull subscription.
type Subscriber struct {
	mu                   sync.RWMutex
	opts                 *subscriberOptions
	pubsubClient         *pubsub.Client
	subscriptionHandlers map[string]*subscriptionHandler
	cancel               context.CancelFunc
}

type subscriptionHandler struct {
	subscription *pubsub.Subscriber
	handleFunc   MessageHandler
}

// MessageHandler defines the message handler invoked by SubscriptionInterceptor to complete the normal
// message handling.
type MessageHandler = func(ctx context.Context, m *pubsub.Message) error

// NewSubscriber initializes new Subscriber.
func NewSubscriber(pubsubClient *pubsub.Client, opt ...SubscriberOption) *Subscriber {
	opts := subscriberOptions{}
	for _, o := range opt {
		o.apply(&opts)
	}
	return &Subscriber{
		opts:                 &opts,
		pubsubClient:         pubsubClient,
		subscriptionHandlers: map[string]*subscriptionHandler{},
	}
}

// HandleSubscriptionFunc registers subscription handler for the given id's subscription.
func (s *Subscriber) HandleSubscriptionFunc(subscription *pubsub.Subscriber, f MessageHandler) error {
	s.mu.RLock()
	if _, ok := s.subscriptionHandlers[subscription.ID()]; ok {
		return fmt.Errorf("handler for subscription '%s' is already registered", subscription.ID())
	}
	s.mu.RUnlock()

	s.mu.Lock()
	s.subscriptionHandlers[subscription.ID()] = &subscriptionHandler{
		subscription: subscription,
		handleFunc:   f,
	}
	s.mu.Unlock()

	return nil
}

// HandleSubscriptionFuncMap registers multiple subscription handlers at once.
func (s *Subscriber) HandleSubscriptionFuncMap(funcMap map[*pubsub.Subscriber]MessageHandler) error {
	eg := errgroup.Group{}
	for sub, f := range funcMap {
		sub, f := sub, f
		eg.Go(func() error {
			return s.HandleSubscriptionFunc(sub, f)
		})
	}
	return eg.Wait()
}

// Run starts running registered pull subscriptions.
func (s *Subscriber) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	for _, handler := range s.subscriptionHandlers {
		h := handler
		subscriptionInfo := SubscriptionInfo{
			SubscriptionID: h.subscription.ID(),
		}
		go func() {
			last := h.handleFunc
			for i := len(s.opts.subscriptionInterceptors) - 1; i >= 0; i-- {
				last = s.opts.subscriptionInterceptors[i](&subscriptionInfo, last)
			}
			err := h.subscription.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
				_ = last(ctx, m)
			})
			if err != nil {
				log.Printf("%+v\n", err)
			}
		}()
	}
}

// Close closes running subscriptions gracefully.
func (s *Subscriber) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}
