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

func runCommand(lg *zap.Logger, s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, errors.New("empty command")
	}
	ss := strings.Split(s, " ")
	if len(ss) == 0 {
		return nil, errors.New("empty command")
	}
	cmd, args := ss[0], ss[1:]
	p, err := exec.New().LookPath(cmd)
	if err != nil {
		return nil, fmt.Errorf("%q does not exist (%v)", p, err)
	}

	lg.Info("running command", zap.String("cmd-path", p), zap.Strings("ars", args))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return exec.New().CommandContext(ctx, p, args...).CombinedOutput()
}
