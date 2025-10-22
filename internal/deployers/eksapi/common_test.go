package eksapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_AZ_PRIORITY(t *testing.T) {
	t.Setenv(AvailabilityZonePriorityEnv, "us-west-2d")
	assert.Equal(t,
		[]string{"us-west-2d", "us-west-2b", "us-west-2c"},
		availabilityZoneHintedOrder([]string{"us-west-2b", "us-west-2c", "us-west-2d"}),
	)
}
