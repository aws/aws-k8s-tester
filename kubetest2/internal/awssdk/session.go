package awssdk

import (
	"github.com/aws/aws-sdk-go/aws/session"
)

// NewSession returns an AWS SDK session with shared config enabled
// it will panic if the session cannot be created
func NewSession() *session.Session {
	return session.Must(
		session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}),
	)
}
