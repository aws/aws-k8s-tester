package eks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func runCommand(lg *zap.Logger, s string, timeout time.Duration) ([]byte, error) {
	if len(s) == 0 {
		return nil, errors.New("empty command")
	}
	args := strings.Split(s, " ")
	if len(args) == 0 {
		return nil, errors.New("empty command")
	}
	p, err := exec.New().LookPath(args[0])
	if err != nil {
		return nil, fmt.Errorf("%q does not exist (%v)", p, err)
	}

	lg.Info("running command", zap.String("command", strings.Join(args, " ")))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	out, err := exec.New().CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	cancel()
	if err == nil {
		lg.Info("ran command")
	} else {
		lg.Warn("failed to run command", zap.Error(err))
	}
	return out, err
}
