package eksconfig

import "testing"

func Test_genClusterName(t *testing.T) {
	id1, id2 := genClusterName(), genClusterName()
	if id1 == id2 {
		t.Fatalf("expected %q != %q", id1, id2)
	}
	t.Log(id1)
	t.Log(id2)
}
