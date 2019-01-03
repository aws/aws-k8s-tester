package kubernetes

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
)

// downloadInstall downloads and installs initial Kubernetes components
func downloadInstall(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	downloads []kubernetesconfig.Download,
	kubeletConfig kubernetesconfig.Kubelet,
	errc chan error,
) {
	instSSH, serr := ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if serr != nil {
		errc <- fmt.Errorf("failed to create a SSH to worker node %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, serr)
		return
	}
	if serr = instSSH.Connect(); serr != nil {
		errc <- fmt.Errorf("failed to connect to worker node %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, serr)
		return
	}
	defer instSSH.Close()

	for _, v := range downloads {
		out, oerr := instSSH.Run(
			v.DownloadCommand,
			ssh.WithTimeout(15*time.Second),
			ssh.WithRetry(3, 3*time.Second),
		)
		if oerr != nil {
			errc <- fmt.Errorf("failed %q to worker node %q(%q) (error %v)", v.DownloadCommand, ec2Config.ClusterName, target.InstanceID, oerr)
			return
		}
		lg.Info(
			"installed at node",
			zap.String("instance-id", target.InstanceID),
			zap.String("path", v.Path),
			zap.String("output", string(out)),
		)
	}

	svc, err := kubeletConfig.Service()
	if err != nil {
		errc <- fmt.Errorf("failed to create Kubelet service file at node (%v)", err)
		return
	}
	var svcPath string
	svcPath, err = fileutil.WriteTempFile([]byte(svc))
	if err != nil {
		errc <- fmt.Errorf("failed to write Kubelet service file at node (%v)", err)
		return
	}
	defer os.RemoveAll(svcPath)
	remotePath := fmt.Sprintf("/home/%s/kubelet.install.sh", ec2Config.UserName)
	_, err = instSSH.Send(
		svcPath,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		errc <- fmt.Errorf("failed to send %q to %q at node %q(%q) (error %v)", svcPath, remotePath, ec2Config.ClusterName, target.InstanceID, err)
		return
	}
	_, err = instSSH.Run(
		fmt.Sprintf("chmod +x %s", remotePath),
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		errc <- fmt.Errorf("failed to 'chmod +x %s' at master node at node %q(%q) (error %v)", remotePath, ec2Config.ClusterName, target.InstanceID, err)
		return
	}
	_, err = instSSH.Run(
		fmt.Sprintf("sudo bash %s", remotePath),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		errc <- fmt.Errorf("failed to run 'sudo bash %s' at node %q(%q) (error %v)", remotePath, ec2Config.ClusterName, target.InstanceID, err)
		return
	}
	lg.Info(
		"wrote kubelet service file at node",
		zap.String("instance-id", target.InstanceID),
	)

	var sysConfigKubelet string
	sysConfigKubelet, err = kubeletConfig.Sysconfig()
	if err != nil {
		errc <- fmt.Errorf("failed to create Kubelet sysconfig at node (%v)", err)
		return
	}
	var sysConfigKubeletPath string
	sysConfigKubeletPath, err = fileutil.WriteTempFile([]byte(sysConfigKubelet))
	if err != nil {
		errc <- fmt.Errorf("failed to write Kubelet sysconfig file at node (%v)", err)
		return
	}
	defer os.RemoveAll(sysConfigKubeletPath)
	remotePath = fmt.Sprintf("/home/%s/kubelet.sysconfig", ec2Config.UserName)
	_, err = instSSH.Send(
		sysConfigKubeletPath,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		errc <- fmt.Errorf("failed to send %q to %q at node %q(%q) (error %v)", sysConfigKubeletPath, remotePath, ec2Config.ClusterName, target.InstanceID, err)
		return
	}
	_, err = instSSH.Run(
		fmt.Sprintf("sudo cp %s /etc/sysconfig/kubelet", remotePath),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		errc <- fmt.Errorf("failed to copy %q to /etc/sysconfig/kubelet at node %q(%q) (error %v)", remotePath, ec2Config.ClusterName, target.InstanceID, err)
		return
	}
	lg.Info(
		"wrote kubelet environment file at node",
		zap.String("instance-id", target.InstanceID),
	)

	errc <- nil
}
