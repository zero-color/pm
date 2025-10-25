package pm

// SubscriptionInfo contains various info about the subscriber.
type SubscriptionInfo struct {
	SubscriptionID string
}

// SubscriptionInterceptor provides a hook to intercept the execution of a message handling.
type SubscriptionInterceptor = func(info *SubscriptionInfo, next MessageHandler) MessageHandler
