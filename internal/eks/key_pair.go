package eks

import (
	"fmt"
	"strings"
)

// isKeyPairDeletedAWSCLI returns true if error indicates that
// the key has already been deleted.
func isKeyPairDeletedAWSCLI(err error, keyName string) bool {
	if err == nil {
		return false
	}
	// An error occurred (InvalidKeyPair.NotFound) when calling the DescribeKeyPairs operation: The key pair 'leegyuho-invalid' does not exist
	return strings.Contains(err.Error(), fmt.Sprintf("An error occurred (InvalidKeyPair.NotFound) when calling the DescribeKeyPairs operation: The key pair '%s' does not exist", keyName))
}

// isKeyPairDeletedGoClient returns true if error indicates that
// the key has already been deleted.
func isKeyPairDeletedGoClient(err error, keyName string) bool {
	if err == nil {
		return false
	}
	// InvalidKeyPair.NotFound: The key pair 'awstester-20180915-TESTID-liROD5s-KEY-PAIR' does not exist
	return strings.Contains(err.Error(), fmt.Sprintf("InvalidKeyPair.NotFound: The key pair '%s' does not exist", keyName))
}
