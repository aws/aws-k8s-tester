package metadata

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestMetadata(t *testing.T) {
	t.Skip()

	s, err := InstanceID(zap.NewExample(), time.Second)
	fmt.Println(s, err)
}
