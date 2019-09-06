package pkg

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type Step interface {
	Run(ctx context.Context) error
}

type TestStep struct {
	Script string
	TestId string
}

// TestStep run execute a user provided script using sh
// It substitutes script variable TEST_ID and set `eu` flag
// so that the script exits when the first error happens
// or there is an undefined variable during execution.
// When error occurs, it terminates all the processes within
// the process group
func (s *TestStep) Run(ctx context.Context) error {
	script := strings.Replace(s.Script, "{{TEST_ID}}", s.TestId, -1)
	cmd := exec.Command("sh", "-eu", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
			return
		case <-ctx.Done():
			if cmd.Process != nil {
				syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		}
	}()

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

type FuncStep struct {
	f func(ctx context.Context) error
}

func (s *FuncStep) Run(ctx context.Context) error {
	return s.f(ctx)
}

func EmptyStep() Step {
	return &FuncStep{func(ctx context.Context) error {
		return nil
	}}
}
