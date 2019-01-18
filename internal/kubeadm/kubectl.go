package kubeadm

import (
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"go.uber.org/zap"
)

func fetchKubeconfig(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
) (kubeconfigOutput []byte, err error) {
	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       ec2Config.KeyPath,
		PublicIP:      target.PublicIP,
		PublicDNSName: target.PublicDNSName,
		UserName:      ec2Config.UserName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to %q(%q) (error %v)", ec2Config.ClusterName, target.InstanceID, err)
	}
	defer ss.Close()

	var nodesOutput []byte
	nodesOutput, err = ss.Run(
		"kubectl --kubeconfig=/home/ec2-user/.kube/config get nodes",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return nil, err
	}
	fmt.Println("nodesOutput:", string(nodesOutput))

	lg.Info("fetching KUBECONFIG")
	kubeconfigOutput, err = ss.Run(
		"cat /home/ec2-user/.kube/config",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return kubeconfigOutput, nil
}
