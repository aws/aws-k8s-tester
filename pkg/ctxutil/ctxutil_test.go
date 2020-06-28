package ctxutil

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTimeLeftTillDeadline(t *testing.T) {
	fmt.Println(TimeLeftTillDeadline(context.TODO()))
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour+time.Second+555*time.Millisecond)
	fmt.Println(TimeLeftTillDeadline(ctx))
	cancel()
	fmt.Println(TimeLeftTillDeadline(ctx))
}
