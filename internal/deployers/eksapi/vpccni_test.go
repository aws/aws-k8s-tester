package eksapi

import (
	"encoding/json"
	"testing"
)

func Test_validVPCCNIDaemonSetPatch(t *testing.T) {
	var j json.RawMessage
	if err := json.Unmarshal([]byte(vpcCNIDaemonSetPatch), &j); err != nil {
		t.Error(err)
	}
}
