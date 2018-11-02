package eks

import (
	"fmt"
	"testing"
)

func TestVPCTemplate(t *testing.T) {
	v := vpcStack{
		Description:       "test",
		Tag:               "aws-k8s-tester",
		TagValue:          "aws-k8s-tester",
		Hostname:          "hostname",
		SecurityGroupName: "security",
	}
	s, err := createVPCTemplate(v)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s)
}
