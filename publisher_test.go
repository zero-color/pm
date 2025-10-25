package pm

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	pb "cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
)

type publisherOptionForTest struct {
}

func (s publisherOptionForTest) apply(so *publisherOptions) {
	so.publishInterceptors = []PublishInterceptor{nil}
}

func TestNewPublisher(t *testing.T) {
	t.Parallel()

	type args struct {
		pubsubClient *pubsub.Client
		opt          []PublisherOption
	}
	tests := []struct {
		name string
		args args
		want *Publisher
	}{
		{
			name: "Initialize new subscriber",
			args: args{
				pubsubClient: &pubsub.Client{},
				opt:          []PublisherOption{publisherOptionForTest{}},
			},
			want: &Publisher{
				opts:   &publisherOptions{publishInterceptors: []PublishInterceptor{nil}},
				Client: &pubsub.Client{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPublisher(tt.args.pubsubClient, tt.args.opt...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPublisher() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPublisher_Publish(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	ts := NewTestServer(ctx, t)
	defer ts.Close()

	topicName := fmt.Sprintf("projects/test-project/topics/TestPublisher_Publish_%d", time.Now().Unix())
	topicPb, err := ts.Client.TopicAdminClient.CreateTopic(ctx, &pb.Topic{
		Name: topicName,
	})
	if err != nil {
		t.Fatal(err)
	}
	publisher := ts.Client.Publisher(topicPb.Name)

	subName := fmt.Sprintf("projects/test-project/subscriptions/TestPublisher_Publish_%d", time.Now().Unix())
	subPb, err := ts.Client.SubscriptionAdminClient.CreateSubscription(ctx, &pb.Subscription{
		Name:  subName,
		Topic: topicPb.Name,
	})
	if err != nil {
		t.Fatal(err)
	}
	subscriber := ts.Client.Subscriber(subPb.Name)

	type args struct {
		ctx       context.Context
		publisher *pubsub.Publisher
		m         *pubsub.Message
	}
	tests := []struct {
		name      string
		publisher *Publisher
		args      args
		wantData  string
		wantErr   bool
	}{
		{
			name:      "Publish message",
			publisher: NewPublisher(ts.Client),
			args:      args{ctx: context.Background(), publisher: publisher, m: &pubsub.Message{Data: []byte("test")}},
			wantData:  "test",
		},
		{
			name: "Publish message with interceptors",
			publisher: NewPublisher(ts.Client, WithPublishInterceptor(func(next MessagePublisher) MessagePublisher {
				return func(ctx context.Context, publisher *pubsub.Publisher, m *pubsub.Message) *pubsub.PublishResult {
					m.Data = []byte("overwritten by first interceptor")
					return next(ctx, publisher, m)
				}
			}, func(next MessagePublisher) MessagePublisher {
				return func(ctx context.Context, publisher *pubsub.Publisher, m *pubsub.Message) *pubsub.PublishResult {
					m.Data = []byte("overwritten by last interceptor")
					return next(ctx, publisher, m)
				}
			})),
			args:     args{ctx: context.Background(), publisher: publisher, m: &pubsub.Message{Data: []byte("test")}},
			wantData: "overwritten by last interceptor",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.publisher.Publish(tt.args.ctx, tt.args.publisher, tt.args.m).Get(tt.args.ctx); !reflect.DeepEqual(err != nil, tt.wantErr) {
				t.Errorf("Publish() = %v, want %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				wg := sync.WaitGroup{}
				wg.Add(1)
				ctx, cancel := context.WithTimeout(tt.args.ctx, 3*time.Second)
				go func() {
					defer wg.Done()
					err := subscriber.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
						m.Ack()
						if string(m.Data) != tt.wantData {
							t.Errorf("publish() gotData = %v, want %v", string(m.Data), tt.wantData)
						}
					})
					if err != nil {
						t.Fatal(err)
					}
				}()
				wg.Wait()
				cancel()
			}
		})
	}

	NewSubscriber(ts.Client)
}
