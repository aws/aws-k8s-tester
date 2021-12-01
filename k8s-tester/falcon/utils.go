package falcon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

func (ts *tester) deploymentExists(ctx context.Context, ns, deploymentName string) (bool, error) {
	inCtx, cancel := context.WithTimeout(ctx, time.Minute)
	_, err := ts.cfg.Client.KubernetesClient().
		AppsV1().
		Deployments(ns).
		Get(inCtx, deploymentName, meta_v1.GetOptions{})
	cancel()
	if errors.IsNotFound(err) {
		return false, nil
	}
	return true, err
}

func (ts *tester) waitForJob(ctx context.Context, ns, jobName string, timeout time.Duration) error {
	inCtx, cancel := context.WithTimeout(ctx, timeout)
	_, _, err := client.WaitForJobCompletes(inCtx, ts.cfg.Logger, ts.cfg.LogWriter, ts.cfg.Stopc, ts.cfg.Client.KubernetesClient(),
		15*time.Second, 5*time.Second,
		ns, jobName,
		1,
	)
	cancel()
	return err
}
func (ts *tester) waitForDeployment(ctx context.Context, ns, deploymentName string, timeout time.Duration) error {
	inCtx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err := client.WaitForDeploymentAvailables(inCtx, ts.cfg.Logger, ts.cfg.LogWriter, ts.cfg.Stopc, ts.cfg.Client.KubernetesClient(),
		0, 5*time.Second,
		ns,
		deploymentName,
		1,
	)

	cancel()
	return err
}

func (ts *tester) kubectlFile(ctx context.Context, operation, file string, timeout time.Duration) error {
	args := []string{
		operation,
		"--filename=" + file,
	}
	return ts.kubectl(ctx, args, timeout)
}

func (ts *tester) kubectl(ctx context.Context, cargs []string, timeout time.Duration) error {
	args := append([]string{
		ts.cfg.Client.Config().KubectlPath,
		"--kubeconfig=" + ts.cfg.Client.Config().KubeconfigPath,
	}, cargs...)

	context, cancel := context.WithTimeout(ctx, timeout)
	output, err := exec.New().CommandContext(context, args[0], args[1:]...).CombinedOutput()
	cancel()
	out := strings.TrimSpace(string(output))
	fmt.Fprintf(ts.cfg.LogWriter, "\n\n'%s' output:\n\n%s\n\n", strings.Join(args, " "), out)
	if err != nil {
		ts.cfg.Logger.Warn("'kubectl'", zap.Error(err))
		ts.cfg.Logger.Warn("Spinnaker Tests::", zap.String("TEST", "FAILED"))
		return err
	}
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
