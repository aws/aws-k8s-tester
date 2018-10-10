package eks

import (
	"fmt"
	"strings"
)

// isCFCreateFailed return true if cloudformation status indicates its creation failure.
func isCFCreateFailed(status string) bool {
	/*
		https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html

		CREATE_IN_PROGRESS
		CREATE_FAILED
		CREATE_COMPLETE
		ROLLBACK_IN_PROGRESS
		ROLLBACK_FAILED
		ROLLBACK_COMPLETE
		DELETE_IN_PROGRESS
		DELETE_FAILED
		DELETE_COMPLETE
		UPDATE_IN_PROGRESS
		UPDATE_COMPLETE_CLEANUP_IN_PROGRESS
		UPDATE_COMPLETE
		UPDATE_ROLLBACK_IN_PROGRESS
		UPDATE_ROLLBACK_FAILED
		UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS
		UPDATE_ROLLBACK_COMPLETE
		REVIEW_IN_PROGRESS
	*/
	if strings.HasPrefix(status, "REVIEW_") || strings.HasPrefix(status, "CREATE_") {
		return false
	}
	return true
}

// isCFDeletedGoClient returns true if cloudformation errror indicates
// that the stack has already been deleted.
func isCFDeletedGoClient(clusterName string, err error) bool {
	if err == nil {
		return false
	}
	// ValidationError: Stack with id AWSTESTER-155460CAAC98A17003-CF-STACK-VPC does not exist\n\tstatus code: 400, request id: bf45410b-b863-11e8-9550-914acc220b7c
	notExistErr := fmt.Sprintf(`ValidationError: Stack with id %s does not exist`, clusterName)
	return strings.Contains(err.Error(), notExistErr)
}
