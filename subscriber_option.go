package pm

type subscriberOptions struct {
	subscriptionInterceptors []SubscriptionInterceptor
}

// SubscriberOption is a option to change subscriber configuration.
type SubscriberOption interface {
	apply(*subscriberOptions)
}

type subscriberOptionFunc struct {
	f func(*subscriberOptions)
}

func (s *subscriberOptionFunc) apply(so *subscriberOptions) {
	s.f(so)
}

func newSubscriberOptionFunc(f func(*subscriberOptions)) *subscriberOptionFunc {
	return &subscriberOptionFunc{
		f: f,
	}
}

// WithSubscriptionInterceptor sets subscriber interceptors.
func WithSubscriptionInterceptor(interceptors ...SubscriptionInterceptor) SubscriberOption {
	return newSubscriberOptionFunc(func(so *subscriberOptions) {
		so.subscriptionInterceptors = interceptors
	})
}
