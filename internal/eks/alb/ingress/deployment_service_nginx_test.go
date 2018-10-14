package ingress

import (
	"fmt"
	"testing"
)

func TestCreateDeploymentServiceNginx(t *testing.T) {
	d, err := CreateDeploymentServiceNginx(ConfigNginx{
		Replicas:  10,
		Namespace: "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(d)
}
