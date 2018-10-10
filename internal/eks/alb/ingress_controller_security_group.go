package alb

import (
	"fmt"
	"strings"
)

// isSecurityGroupHasDependencyGoClient returns true if error indicates that
// the security group has dependency thus cannot be deleted at the moment.
func isSecurityGroupHasDependencyGoClient(err error, sgID string) bool {
	if err == nil {
		return false
	}
	// DependencyViolation: resource sg-0e51815d7fc1e1c8d has a dependent object
	return strings.Contains(err.Error(), fmt.Sprintf("DependencyViolation: resource %s has a dependent object", sgID))
}

func isSecurityGroupDeletedGoClient(err error) bool {
	if err == nil {
		return false
	}
	// InvalidGroup.NotFound: The security group 'sg-0dcf748f4901c5334' does not exist
	// InvalidPermission.NotFound: The specified rule does not exist in this security group.
	return strings.Contains(err.Error(), "does not exist")
}
