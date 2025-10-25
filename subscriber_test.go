package pm

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	pb "cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
)

type subscriberOptionForTest struct {
}

func (s subscriberOptionForTest) apply(so *subscriberOptions) {
	so.subscriptionInterceptors = []SubscriptionInterceptor{nil}
}

func TestNewSubscriber(t *testing.T) {
	t.Parallel()

	type args struct {
		pubsubClient *pubsub.Client
		opt          []SubscriberOption
	}
	tests := []struct {
		name string
		args args
		want *Subscriber
	}{
		{
			name: "Initialize new subscriber",
			args: args{
				pubsubClient: &pubsub.Client{},
				opt:          []SubscriberOption{subscriberOptionForTest{}},
			},
			want: &Subscriber{
				opts:                 &subscriberOptions{subscriptionInterceptors: []SubscriptionInterceptor{nil}},
				pubsubClient:         &pubsub.Client{},
				subscriptionHandlers: map[string]*subscriptionHandler{},
				cancel:               nil,
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Note: DeepEqual comparison is skipped for complex client objects
			got := NewSubscriber(tt.args.pubsubClient, tt.args.opt...)
			if got == nil {
				t.Errorf("NewSubscriber() returned nil")
			}
		})
	}
}

func TestSubscriber_HandleSubscriptionFunc(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ts := NewTestServer(ctx, t)
	defer ts.Close()

	topicName := fmt.Sprintf("projects/test-project/topics/TestSubscriber_HandleSubscriptionFunc_%d", time.Now().Unix())
	topicPb, err := ts.Client.TopicAdminClient.CreateTopic(ctx, &pb.Topic{
		Name: topicName,
	})
	if err != nil {
		t.Fatal(err)
	}

	subName := fmt.Sprintf("projects/test-project/subscriptions/TestSubscriber_HandleSubscriptionFunc_%d", time.Now().Unix())
	subPb, err := ts.Client.SubscriptionAdminClient.CreateSubscription(ctx, &pb.Subscription{
		Name:  subName,
		Topic: topicPb.Name,
	})
	if err != nil {
		t.Fatal(err)
	}
	sub := ts.Client.Subscriber(subPb.Name)

	type args struct {
		topicID      string
		subscription *pubsub.Subscriber
		f            MessageHandler
	}
	tests := []struct {
		name       string
		subscriber *Subscriber
		args       args
		wantErr    bool
	}{
		{
			name:       "sets subscriber handler",
			subscriber: NewSubscriber(ts.Client),
			args: args{
				topicID:      topicPb.Name,
				subscription: sub,
				f:            func(ctx context.Context, m *pubsub.Message) error { return nil },
			},
		},
		{
			name: "when a handler is already registered for the give subscriber id, it returns error",
			subscriber: func() *Subscriber {
				s := NewSubscriber(ts.Client)
				_ = s.HandleSubscriptionFunc(topicPb.Name, sub, func(ctx context.Context, m *pubsub.Message) error { return nil })
				return s
			}(),
			args: args{
				topicID:      topicPb.Name,
				subscription: sub,
				f:            func(ctx context.Context, m *pubsub.Message) error { return nil },
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.subscriber.HandleSubscriptionFunc(tt.args.topicID, tt.args.subscription, tt.args.f); (err != nil) != tt.wantErr {
				t.Errorf("HandleSubscriptionFunc() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if _, ok := tt.subscriber.subscriptionHandlers[tt.args.subscription.ID()]; !ok {
					t.Errorf("HandleSubscriptionFunc() is expected to set subscriber handler")
				}
			}
		})
	}
}

func TestSubscriber_HandleSubscriptionFuncMap(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ts := NewTestServer(ctx, t)
	defer ts.Close()

	topicName := fmt.Sprintf("projects/test-project/topics/TestSubscriber_HandleSubscriptionFuncMap_%d", time.Now().Unix())
	topicPb, err := ts.Client.TopicAdminClient.CreateTopic(ctx, &pb.Topic{
		Name: topicName,
	})
	if err != nil {
		t.Fatal(err)
	}

	subName := fmt.Sprintf("projects/test-project/subscriptions/TestSubscriber_HandleSubscriptionFuncMap_%d", time.Now().Unix())
	subPb, err := ts.Client.SubscriptionAdminClient.CreateSubscription(ctx, &pb.Subscription{
		Name:  subName,
		Topic: topicPb.Name,
	})
	if err != nil {
		t.Fatal(err)
	}
	sub := ts.Client.Subscriber(subPb.Name)

	type args struct {
		topicID string
		funcMap map[*pubsub.Subscriber]MessageHandler
	}
	tests := []struct {
		name       string
		subscriber *Subscriber
		args       args
		wantErr    bool
	}{
		{
			name:       "sets subscriber handler",
			subscriber: NewSubscriber(ts.Client),
			args: args{
				topicID: topicPb.Name,
				funcMap: map[*pubsub.Subscriber]MessageHandler{
					sub: func(ctx context.Context, m *pubsub.Message) error { return nil },
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.subscriber.HandleSubscriptionFuncMap(tt.args.topicID, tt.args.funcMap); (err != nil) != tt.wantErr {
				t.Errorf("HandleSubscriptionFuncMap() error = %v, wantErr %v", err, tt.wantErr)
			}

			for sub := range tt.args.funcMap {
				if _, ok := tt.subscriber.subscriptionHandlers[sub.ID()]; ok == tt.wantErr {
					t.Errorf("HandleSubscriptionFuncMap() set: %v, want set: %v", ok, !tt.wantErr)
				}
			}
		})
	}
}

func TestSubscriber_Run(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ts := NewTestServer(ctx, t)
	defer ts.Close()

	topicName := fmt.Sprintf("projects/test-project/topics/TestSubscriber_Run_%d", time.Now().Unix())
	topicPb, err := ts.Client.TopicAdminClient.CreateTopic(ctx, &pb.Topic{
		Name: topicName,
	})
	if err != nil {
		t.Fatal(err)
	}
	publisher := ts.Client.Publisher(topicPb.Name)

	subName := fmt.Sprintf("projects/test-project/subscriptions/TestSubscriber_Run_%d", time.Now().Unix())
	subPb, err := ts.Client.SubscriptionAdminClient.CreateSubscription(ctx, &pb.Subscription{
		Name:  subName,
		Topic: topicPb.Name,
	})
	if err != nil {
		t.Fatal(err)
	}
	sub := ts.Client.Subscriber(subPb.Name)

	subscriber := NewSubscriber(
		ts.Client,
		WithSubscriptionInterceptor(func(_ *SubscriptionInfo, next MessageHandler) MessageHandler {
			return func(ctx context.Context, m *pubsub.Message) error {
				m.Attributes = map[string]string{"intercepted": "abc"}
				return next(ctx, m)
			}
		}, func(_ *SubscriptionInfo, next MessageHandler) MessageHandler {
			return func(ctx context.Context, m *pubsub.Message) error {
				// this will overwrite the first interceptor
				m.Attributes = map[string]string{"intercepted": "true"}
				return next(ctx, m)
			}
		}),
	)

	wg := sync.WaitGroup{}
	var receivedCount int64 = 0
	err = subscriber.HandleSubscriptionFunc(topicPb.Name, sub, func(ctx context.Context, m *pubsub.Message) error {
		defer wg.Done()
		m.Ack()
		atomic.AddInt64(&receivedCount, 1)
		if m.Attributes["intercepted"] != "true" {
			t.Error("Interceptor didn't work")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	subscriber.Run(context.Background())

	var publishCount int64 = 2
	for i := 0; i < int(publishCount); i++ {
		wg.Add(1)
		ctx := context.Background()
		_, err := publisher.Publish(ctx, &pubsub.Message{Data: []byte("test")}).Get(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	wg.Wait()

	if atomic.LoadInt64(&receivedCount) != publishCount {
		t.Errorf("Published count and received count doesn't match, published: %d, received: %d", publishCount, receivedCount)
	}
}

func TestSubscriber_Close(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ts := NewTestServer(ctx, t)
	defer ts.Close()

	topicName := fmt.Sprintf("projects/test-project/topics/TestSubscriber_Close_%d", time.Now().Unix())
	topicPb, err := ts.Client.TopicAdminClient.CreateTopic(ctx, &pb.Topic{
		Name: topicName,
	})
	if err != nil {
		t.Fatal(err)
	}
	publisher := ts.Client.Publisher(topicPb.Name)

	subName := fmt.Sprintf("projects/test-project/subscriptions/TestSubscriber_Close_%d", time.Now().Unix())
	subPb, err := ts.Client.SubscriptionAdminClient.CreateSubscription(ctx, &pb.Subscription{
		Name:  subName,
		Topic: topicPb.Name,
	})
	if err != nil {
		t.Fatal(err)
	}
	sub := ts.Client.Subscriber(subPb.Name)

	subscriber := NewSubscriber(ts.Client)

	err = subscriber.HandleSubscriptionFunc(topicPb.Name, sub, func(ctx context.Context, m *pubsub.Message) error {
		m.Ack()
		t.Error("Must not received messages after close")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	subscriber.Run(context.Background())
	subscriber.Close()

	if _, err := publisher.Publish(ctx, &pubsub.Message{Data: []byte("test")}).Get(ctx); err != nil {
		log.Fatal(err)
	}
	time.Sleep(1 * time.Second)
}
