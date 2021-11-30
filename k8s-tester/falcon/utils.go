package falcon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	"k8s.io/utils/exec"
)

func (ts *tester) waitForDeployment(ctx context.Context, ns, deploymentName string, timeout time.Duration) error {
	inCtx, cancel := context.WithTimeout(context.Background(), timeout)
	deployment, err := client.WaitForDeploymentAvailables(inCtx, ts.cfg.Logger, ts.cfg.LogWriter, ts.cfg.Stopc, ts.cfg.Client.KubernetesClient(),
		0, 5*time.Second,
		ns,
		deploymentName,
		1,
	)

	cancel()
	if err != nil {
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "%v", deployment.Status)
	return nil
}

func (ts *tester) kubectl(ctx context.Context, operation, file string, timeout time.Duration) error {
	args := []string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
		operation,
		"--filename=" + file,
	}

	context, cancel := context.WithTimeout(ctx, timeout)
	output, err := exec.New().CommandContext(context, args[0], args[1:]...).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	if err != nil {
		ts.cfg.Logger.Warn(fmt.Sprintf("'kubectl %s' failed", operation), zap.Error(err))
		ts.cfg.Logger.Warn("Spinnaker Tests::", zap.String("TEST", "FAILED"))
		return err
	}
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", strings.Join(args, " "), out)
	return nil

}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources, should we continue?", action)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}
