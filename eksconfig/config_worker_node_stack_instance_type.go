package eksconfig

import "github.com/aws/awstester/pkg/awsapi/ec2"

func checkEC2InstanceType(s string) (ok bool) {
	_, ok = ec2.InstanceTypes[s]
	return ok
}
