package eksconfig

import (
	"testing"
)

func TestConfig(t *testing.T) {
	cfg := NewDefault()

	// supports only 58 pods per node
	cfg.WorkerNodeInstanceType = "m5.xlarge"
	cfg.WorkderNodeASGMin = 10
	cfg.WorkderNodeASGMax = 10
	cfg.ALBIngressController.TestServerReplicas = 600

	err := cfg.ValidateAndSetDefaults()
	if err == nil {
		t.Fatal("expected error")
	}
	t.Log(err)
}

func Test_genClusterName(t *testing.T) {
	id1, id2 := genClusterName(genTag()), genClusterName(genTag())
	if id1 == id2 {
		t.Fatalf("expected %q != %q", id1, id2)
	}
	t.Log(id1)
	t.Log(id2)
}
