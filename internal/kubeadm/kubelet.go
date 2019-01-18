package kubeadm

import (
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubeadmconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
)

func writeKubeletEnvFile(kubeletConfig kubeadmconfig.Kubelet) (p string, err error) {
	var sc string
	sc, err = kubeletConfig.Sysconfig()
	if err != nil {
		return "", fmt.Errorf("failed to create kubelet sysconfig (%v)", err)
	}
	p, err = fileutil.WriteTempFile([]byte(sc))
	if err != nil {
		return "", fmt.Errorf("failed to write kubelet sysconfig file (%v)", err)
	}
	return p, nil
}

func sendKubeletEnvFile(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	filePathToSend string,
) (err error) {
	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	defer ss.Close()

	remotePath := fmt.Sprintf("/home/%s/kubelet.sysconfig", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	copyCmd := fmt.Sprintf("sudo mkdir -p /etc/sysconfig/ && sudo cp %s /etc/sysconfig/kubelet", remotePath)
	_, err = ss.Run(
		copyCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", copyCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	catCmd := "sudo cat /etc/sysconfig/kubelet"
	var out []byte
	out, err = ss.Run(
		catCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil || len(out) == 0 {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", catCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	return nil
}

func startKubeletService(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
) (err error) {
	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	defer ss.Close()

	remoteCmd := "sudo systemctl daemon-reload"
	_, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	var out []byte
	remoteCmd = "sudo systemctl cat kubelet.service"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	remoteCmd = "sudo systemctl enable kubelet && sudo systemctl restart kubelet"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	time.Sleep(5 * time.Second)
	remoteCmd = "sudo systemctl status kubelet --full --no-pager || true"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	time.Sleep(5 * time.Second)
	remoteCmd = "sudo journalctl --no-pager --output=cat -u kubelet"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	time.Sleep(5 * time.Second)
	remoteCmd = "kubeadm version"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	time.Sleep(5 * time.Second)
	remoteCmd = "kubelet --version"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	time.Sleep(5 * time.Second)
	remoteCmd = "kubectl version --client"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	time.Sleep(5 * time.Second)
	remoteCmd = "crictl --version"
	out, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	lg.Info("ran", zap.String("command", remoteCmd))
	fmt.Println(string(out))

	return nil
}
