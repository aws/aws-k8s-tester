package kubernetes

import (
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"go.uber.org/zap"
)

// download downloads initial Kubernetes components
func download(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	downloads []kubernetesconfig.Download,
	errc chan error,
) {
	instSSH, err := ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		errc <- fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
		return
	}
	if err = instSSH.Connect(); err != nil {
		errc <- fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
		return
	}
	defer instSSH.Close()

	for _, v := range downloads {
		var out []byte
		out, err := instSSH.Run(
			v.DownloadCommand,
			ssh.WithTimeout(15*time.Second),
			ssh.WithRetry(3, 3*time.Second),
		)
		if err != nil {
			errc <- fmt.Errorf("failed %q to %q(%q) (error %v)", v.DownloadCommand, ec2Config.ClusterName, target.InstanceID, err)
			return
		}
		lg.Info(
			"successfully downloaded to a node",
			zap.String("instance-id", target.InstanceID),
			zap.String("path", v.Path),
			zap.String("output", string(out)),
		)
	}

	errc <- nil
	return
}
