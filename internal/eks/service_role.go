package eks

import "strings"

const (
	serviceRolePolicyDoc = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {
				"Service": "eks.amazonaws.com"
			},
			"Action": "sts:AssumeRole"
		}
	]
}`
)

// isIAMRoleDeletedGoClient returns true if error from IAM API call
// indicates that the role has already been deleted or does not exist.
func isIAMRoleDeletedGoClient(err error) bool {
	if err == nil {
		return false
	}
	// NoSuchEntity: The user with name aws-k8s-tester-155479CED0B80EE801-SERVICE-ROLE-POLICY cannot be found.
	// NoSuchEntity: The role with name aws-k8s-tester-20180918-TESTID-4RHA3tT-SERVICE-ROLE cannot be found.
	// TODO: use aweerr.Code
	return strings.Contains(err.Error(), "cannot be found")
}
