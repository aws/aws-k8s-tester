package kubeadm

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubeadmconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
)

func writeKubeadmInitScript(kubeadmConfiog kubeadmconfig.KubeadmInit) (p string, err error) {
	var sc string
	sc, err = kubeadmConfiog.Script()
	if err != nil {
		return "", fmt.Errorf("failed to create kubelet sysconfig (%v)", err)
	}
	p, err = fileutil.WriteTempFile([]byte(sc))
	if err != nil {
		return "", fmt.Errorf("failed to write kubelet sysconfig file (%v)", err)
	}
	return p, nil
}

func sendKubeadmInitScript(
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

	fmt.Println("/etc/sysconfig/kubelet:", string(out))

	return nil
}

func runKubeadmInit(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	filePathToSend string,
	kubeadmJoin *kubeadmconfig.KubeadmJoin,
) (err error) {
	kubeadmJoin.Target = fmt.Sprintf("%s:6443", target.PrivateIP)
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

	remotePath := fmt.Sprintf("/home/%s/kubeadm.init.sh", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
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
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return err
	}
	lg.Info("started kubeadm init", zap.String("id", target.InstanceID))

	retryStart := time.Now().UTC()
joinReady:
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var kubeadmInitOut []byte
		kubeadmInitOut, err = ss.Run(
			"cat /var/log/kubeadm-init.log",
			ssh.WithRetry(15, 5*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		output := string(kubeadmInitOut)
		debugLines := strings.Split(output, "\n")
		lines := make([]string, len(debugLines))
		copy(lines, debugLines)
		if len(debugLines) > 10 {
			debugLines = debugLines[len(debugLines)-10:]
		}
		fmt.Printf("\n\n%s\n\n", strings.Join(debugLines, "\n"))

		if !strings.Contains(output, "kubeadm join ") {
			lg.Info("waiting on kubeadm init")
			time.Sleep(15 * time.Second)
			continue
		}

		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if !strings.Contains(line, "--token") || !strings.Contains(line, "--discovery-token-ca-cert-hash") {
				continue
			}
			kubeadmJoin.RawCommand = line
			break joinReady
		}
	}

	if kubeadmJoin.RawCommand == "" {
		return errors.New("kubeadm join failed")
	}
	prevToken, prevHash := false, false
	for _, field := range strings.Fields(kubeadmJoin.RawCommand) {
		if field == "--token" {
			prevToken = true
			continue
		}
		if prevToken {
			kubeadmJoin.Token = field
			prevToken = false
			continue
		}
		if field == "--discovery-token-ca-cert-hash" {
			prevHash = true
			continue
		}
		if prevHash {
			kubeadmJoin.DiscoveryTokenCACertHash = field
			prevHash = true
			continue
		}
	}

	lg.Info("checking kube-controller-manager")
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var dockerPsOutput []byte
		dockerPsOutput, err = ss.Run(
			"sudo docker ps",
			ssh.WithRetry(15, 5*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		output := string(dockerPsOutput)
		fmt.Printf("\n\n%s\n\n", output)
		if strings.Contains(output, "kube-controller-manager") {
			break
		}
		lg.Info("waiting on kube-controller-manager")
		time.Sleep(15 * time.Second)
	}
	lg.Info("kube-controller-manager is ready")

	lg.Info("checking kube-controller-manager pod")
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var podsOutput []byte
		podsOutput, err = ss.Run(
			"kubectl --kubeconfig=/home/ec2-user/.kube/config get pods --all-namespaces",
			ssh.WithRetry(15, 5*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		if err != nil {
			return err
		}
		fmt.Println("podsOutput:", string(podsOutput))
		if strings.Contains(string(podsOutput), "kube-controller-manager-") {
			break
		}
		time.Sleep(15 * time.Second)
	}
	lg.Info("kube-controller-manager pod is ready")

	var flannelOutputRole []byte
	flannelOutputRole, err = ss.Run(
		"kubectl --kubeconfig=/home/ec2-user/.kube/config apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/k8s-manifests/kube-flannel-rbac.yml",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to apply flannel role (%v)", err)
	}
	fmt.Println("flannelOutputRole:", string(flannelOutputRole))
	var flannelOutput []byte
	flannelOutput, err = ss.Run(
		"kubectl --kubeconfig=/home/ec2-user/.kube/config apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to apply flannel (%v)", err)
	}
	fmt.Println("flannelOutput:", string(flannelOutput))

	lg.Info("checking flannel pod")
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var podsOutput []byte
		podsOutput, err = ss.Run(
			"kubectl --kubeconfig=/home/ec2-user/.kube/config get pods --all-namespaces",
			ssh.WithRetry(15, 5*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		if err != nil {
			return err
		}
		fmt.Println("podsOutput:", string(podsOutput))

		if strings.Contains(string(podsOutput), "kube-flannel-") {
			break
		}
		time.Sleep(15 * time.Second)
	}
	lg.Info("flannel pod is ready")

	return nil
}
