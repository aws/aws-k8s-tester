package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubeadmconfig"
	"go.uber.org/zap"
)

func runKubeadmJoin(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	kubeadmJoin *kubeadmconfig.KubeadmJoin,
) (err error) {
	var joinCmd string
	joinCmd, err = kubeadmJoin.Command()
	if err != nil {
		return err
	}
	lg.Info("kubeadm join command is ready", zap.String("command", joinCmd))

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

	_, err = ss.Run(
		"sudo systemctl enable kubelet.service",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return err
	}
	_, err = ss.Run(
		"sudo systemctl start kubelet.service",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return err
	}
	lg.Info("started kubelet", zap.String("id", target.InstanceID))

	var joinOutput []byte
	joinOutput, err = ss.Run(
		joinCmd,
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(3*time.Minute),
	)
	if err != nil {
		return err
	}
	output := string(joinOutput)
	if !strings.Contains(output, "[discovery] Successfully established connection with API Server") || !strings.Contains(output, "This node has joined the cluster:") {
		return fmt.Errorf("failed to join cluster (%q)", output)
	}
	lg.Info("node has joined master", zap.String("id", target.InstanceID), zap.String("output", string(joinOutput)))

	return nil
}
