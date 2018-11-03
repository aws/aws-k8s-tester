package ec2config

import (
	"fmt"
	"math/rand"
	"regexp"
	"time"
)

// genS3URL returns S3 URL path.
// e.g. https://s3-us-west-2.amazonaws.com/aws-k8s-tester-20180925/hello-world
func genS3URL(region, bucket, s3Path string) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", region, bucket, s3Path)
}

// genTag generates a tag for cluster name, CloudFormation, and S3 bucket.
func genTag() string {
	// use UTC time for everything
	now := time.Now().UTC()
	return fmt.Sprintf("aws-k8s-tester-ec2-%d%02d%02d", now.Year(), now.Month(), now.Day())
}

var reg *regexp.Regexp

func init() {
	var err error
	reg, err = regexp.Compile("[^a-zA-Z]+")
	if err != nil {
		panic(err)
	}
}

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UTC().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
