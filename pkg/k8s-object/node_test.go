package k8sobject

import (
	"fmt"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
)

func TestParseNode(t *testing.T) {
	np1 := ParseNodeInfo(v1.NodeSystemInfo{
		KubeletVersion:   "v1.15.10",
		KubeProxyVersion: "v1.15.10",
	})
	exp1 := NodeInfo{
		NodeSystemInfo: v1.NodeSystemInfo{
			KubeletVersion:   "v1.15.10",
			KubeProxyVersion: "v1.15.10",
		},
		KubeletMinorVersionValue:   1.15,
		KubeProxyMinorVersionValue: 1.15,
	}
	if !reflect.DeepEqual(np1, exp1) {
		t.Fatalf("expected %+v, got %+v", exp1, np1)
	}
	np2 := ParseNodeInfo(v1.NodeSystemInfo{
		KubeletVersion:   "v1.16.8-eks-e16311",
		KubeProxyVersion: "v1.16.8-eks-e16311",
	})
	exp2 := NodeInfo{
		NodeSystemInfo: v1.NodeSystemInfo{
			KubeletVersion:   "v1.16.8-eks-e16311",
			KubeProxyVersion: "v1.16.8-eks-e16311",
		},
		KubeletMinorVersionValue:   1.16,
		KubeProxyMinorVersionValue: 1.16,
	}
	if !reflect.DeepEqual(np2, exp2) {
		t.Fatalf("expected %+v, got %+v", exp2, np2)
	}

	fmt.Printf("%+v\n", np1)
	fmt.Printf("%+v\n", np2)
}
