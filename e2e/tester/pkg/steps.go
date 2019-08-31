package pkg

import (
	"os"
	"os/exec"
	"strings"
)

type Step interface {
	Run() error
}

type TestStep struct {
	Script string
	TestId string
}

func (s *TestStep) Run() error {
	script := strings.Replace(s.Script, "{{TEST_ID}}", s.TestId, -1)
	testCmd := exec.Command("sh", "-c", script)
	testCmd.Stdout = os.Stdout
	testCmd.Stdin = os.Stdin
	testCmd.Stderr = os.Stderr
	err := testCmd.Run()
	if err != nil {
		return err
	}
	return nil
}

type FuncStep struct {
	f func() error
}

func (s *FuncStep) Run() error {
	return s.f()
}

func EmptyStep() Step {
	return &FuncStep{func() error {
		return nil
	}}
}
