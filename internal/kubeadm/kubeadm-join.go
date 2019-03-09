package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubeadmconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
)

func runKubeadmJoin(
	lg *zap.Logger,
	targetEC2 ec2config.Config,
	targetInstance ec2config.Instance,
	kubeadmJoin *kubeadmconfig.KubeadmJoin,
) (err error) {
	kubeadmJoin.WorkerNodePrivateDNS = targetInstance.PrivateDNSName

	var kubeadmJoinScript string
	kubeadmJoinScript, err = kubeadmJoin.Script()
	if err != nil {
		return err
	}
	var kubeadmJoinScriptPath string
	kubeadmJoinScriptPath, err = fileutil.WriteTempFile([]byte(kubeadmJoinScript))
	if err != nil {
		return err
	}

	var ss ssh.SSH
	ss, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       targetEC2.KeyPath,
		PublicIP:      targetInstance.PublicIP,
		PublicDNSName: targetInstance.PublicDNSName,
		UserName:      targetEC2.UserName,
	})
	if err != nil {
		return fmt.Errorf("failed to create a SSH to %q(%q) (error %v)", targetEC2.ClusterName, targetInstance.InstanceID, err)
	}
	if err = ss.Connect(); err != nil {
		return fmt.Errorf("failed to connect to %q(%q) (error %v)", targetEC2.ClusterName, targetInstance.InstanceID, err)
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
	lg.Info("started kubelet", zap.String("id", targetInstance.InstanceID))

	lg.Info("starting 'kubeadm join'", zap.String("id", targetInstance.InstanceID))
	remotePath := fmt.Sprintf("/home/%s/kubeadm.join.sh", targetEC2.UserName)
	_, err = ss.Send(
		kubeadmJoinScriptPath,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", kubeadmJoinScriptPath, remotePath, targetEC2.ClusterName, targetInstance.InstanceID, err)
	}
	_, err = ss.Run(
		fmt.Sprintf("chmod +x %s", remotePath),
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return err
	}
	_, err = ss.Run(
		fmt.Sprintf("sudo bash %s", remotePath),
		ssh.WithTimeout(5*time.Minute),
	)
	if err != nil {
		return err
	}
	lg.Info("started 'kubeadm join'", zap.String("id", targetInstance.InstanceID))

	retryStart := time.Now().UTC()
joinReady:
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var co []byte
		co, err = ss.Run(
			"sudo cat /var/log/kubeadm.join.log",
			ssh.WithRetry(15, 5*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		output := string(co)
		debugLines := strings.Split(output, "\n")
		lines := make([]string, len(debugLines))
		copy(lines, debugLines)
		if len(debugLines) > 10 {
			debugLines = debugLines[len(debugLines)-10:]
		}
		fmt.Printf("\n\n%s\n\n", strings.Join(debugLines, "\n"))

		if strings.Contains(output, "[discovery] Successfully established connection with API Server") ||
			strings.Contains(output, "This node has joined the cluster:") {
			break joinReady
		}

		time.Sleep(7 * time.Second)
	}

	lg.Info("node has joined master", zap.String("id", targetInstance.InstanceID))
	return nil
}
