package spinner

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSpinner(t *testing.T) {
	s := New(os.Stderr, "hello")
	s.Restart()
	time.Sleep(3 * time.Second)
	s.Stop()
	fmt.Println("hello")
}
