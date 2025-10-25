package pm_logging

import "github.com/zero-color/pm"

// LogDecider function defines rules for suppressing any interceptor logs
type LogDecider func(info *pm.SubscriptionInfo, err error) bool

// DefaultLogDecider is the default implementation of decider to see if you should log the call
// by default this if always true so all processing are logged
func DefaultLogDecider(_ *pm.SubscriptionInfo, _ error) bool {
	return true
}
