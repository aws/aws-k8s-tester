package eks

import (
	"fmt"
	"strings"
	"testing"
)

func TestWorkerNodeTemplate(t *testing.T) {
	v := workerNodeStack{
		Description:                          "test",
		Tag:                                  "aws-k8s-tester",
		TagValue:                             "aws-k8s-tester",
		Hostname:                             "hostname",
		EnableWorkerNodeSSH:                  true,
		EnableWorkerNodePrivilegedPortAccess: false,
	}
	s, err := _createWorkerNodeTemplate(v)
	if err != nil {
		t.Fatal(err)
	}
	if v.EnableWorkerNodeSSH && !strings.Contains(s, "ClusterControlPlaneSecurityGroupIngress22") {
		t.Fatal("expected 'ClusterControlPlaneSecurityGroupIngress22'")
	}
	if v.EnableWorkerNodePrivilegedPortAccess && !strings.Contains(s, "FromPort: 1025") {
		t.Fatal("expected 'NodeSecurityGroupFromControlPlaneIngress' and 'ControlPlaneEgressToNodeSecurityGroup' to have FromPort = 1025")
	}
	fmt.Println(s)
}
