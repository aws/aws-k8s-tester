package eks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/httputil"

	"go.uber.org/zap"
)

// https://github.com/aws/amazon-vpc-cni-k8s/releases
func (md *embedded) upgradeCNI() error {
	d, err := httputil.Download(md.lg, os.Stdout, "https://raw.githubusercontent.com/aws/amazon-vpc-cni-k8s/master/config/v1.2/aws-k8s-cni.yaml")
	if err != nil {
		return err
	}
	var f *os.File
	f, err = ioutil.TempFile(os.TempDir(), "cni-v1.2.yaml")
	if err != nil {
		return err
	}
	if _, err = f.Write(d); err != nil {
		return err
	}
	cmPath := f.Name()
	f.Close()

	kcfgPath := md.cfg.KubeConfigPath
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		select {
		case <-md.stopc:
			return nil
		default:
		}

		// TODO: use "k8s.io/client-go"
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+kcfgPath,
			"apply", "--filename="+cmPath,
		)
		var kexo []byte
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			if strings.Contains(err.Error(), "unknown flag:") {
				return fmt.Errorf("unknown flag %s", string(kexo))
			}
			md.lg.Warn("failed to upgrade CNI",
				zap.String("output", string(kexo)),
				zap.Error(err),
			)

			time.Sleep(5 * time.Second)
			continue
		}

		md.lg.Info("upgraded CNI using kubectl", zap.String("output", string(kexo)))
		break
	}

	return nil
}
