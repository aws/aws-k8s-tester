package timeutil

import (
	"fmt"
	"testing"
	"time"
)

func TestTimeFrame(t *testing.T) {
	fmt.Printf("%+v\n", NewTimeFrame(time.Now(), time.Now().Add(time.Hour)))
}
