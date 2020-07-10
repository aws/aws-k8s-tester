package spinner

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSpinner(t *testing.T) {
	s := New("hello", os.Stderr)
	s.Start()
	time.Sleep(3 * time.Second)
	s.Stop()
	fmt.Println("hello")
}
