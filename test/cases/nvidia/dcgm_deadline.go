package nvidia

import (
	"fmt"
	"time"
)

// dcgmDiagDeadlineSlack is how far inside the wait timeout the Job's own
// deadline sits. Bounding the Job first means a hung run surfaces as a Job
// failure with logs attached, rather than as an opaque wait timeout.
const dcgmDiagDeadlineSlack = time.Minute

// minDcgmDiagTimeoutMinutes is the smallest wait timeout that still yields a
// positive activeDeadlineSeconds. Kubernetes rejects a Job whose
// activeDeadlineSeconds is not positive, so anything lower cannot produce a
// valid Job.
const minDcgmDiagTimeoutMinutes = 2

// dcgmDiagDeadlineSeconds converts a wait timeout in minutes into the Job's
// activeDeadlineSeconds, rejecting values that cannot render a valid Job.
//
// Validation and arithmetic live together deliberately: keeping them apart
// invites a bounds check that no longer matches the expression it guards.
//
// This file carries no build tag so the logic can be unit tested without the
// e2e TestMain, which requires a live cluster.
func dcgmDiagDeadlineSeconds(timeoutMinutes int) (int, error) {
	if timeoutMinutes < minDcgmDiagTimeoutMinutes {
		return 0, fmt.Errorf("dcgmDiagTimeoutMinutes must be at least %d, got %d",
			minDcgmDiagTimeoutMinutes, timeoutMinutes)
	}
	deadline := int((time.Duration(timeoutMinutes)*time.Minute - dcgmDiagDeadlineSlack).Seconds())
	if deadline <= 0 {
		// Unreachable given the bound above; kept so a change to either the
		// bound or the slack cannot silently emit an invalid Job.
		return 0, fmt.Errorf("dcgmDiagTimeoutMinutes=%d yields activeDeadlineSeconds=%d, which Kubernetes rejects",
			timeoutMinutes, deadline)
	}
	return deadline, nil
}
