package eks

import (
	"fmt"
	"testing"
)

func TestVPCTemplate(t *testing.T) {
	v := vpcStack{
		Description:       "test",
		TagKey:            "awstester",
		TagValue:          "awstester",
		Hostname:          "hostname",
		SecurityGroupName: "security",
	}
	s, err := createVPCTemplate(v)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s)
}
