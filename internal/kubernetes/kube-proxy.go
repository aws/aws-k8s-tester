package kubernetes

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

func writeKubeProxyKubeConfigFile(
	privateKey []byte,
	publicKey []byte,
	rootCA []byte,
	internalServerURL string,
) (p string, err error) {
	cfg := clientcmdapi.NewConfig()
	cfg.APIVersion = "v1"
	cfg.Kind = "Config"
	cfg.Clusters["local"] = &clientcmdapi.Cluster{
		CertificateAuthorityData: rootCA,
		Server:                   internalServerURL,
	}
	cfg.Contexts["service-account-context"] = &clientcmdapi.Context{
		Cluster:  "local",
		AuthInfo: "kube-proxy",
	}
	cfg.CurrentContext = "service-account-context"
	cfg.AuthInfos["kube-proxy"] = &clientcmdapi.AuthInfo{
		ClientCertificateData: publicKey,
		ClientKeyData:         privateKey,
	}
	var d []byte
	d, err = yaml.Marshal(&cfg)
	if err != nil {
		return "", err
	}
	p, err = fileutil.WriteTempFile(d)
	if err != nil {
		return "", fmt.Errorf("failed to write kube-proxy KUBECONFIG file (%v)", err)
	}
	return p, nil
}

func sendKubeProxyKubeConfigFile(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	filePathToSend string,
	kubeProxyConfig kubernetesconfig.KubeProxy,
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

	remotePath := fmt.Sprintf("/home/%s/kube-proxy.kubeconfig", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	copyCmd := fmt.Sprintf("sudo mkdir -p %s && sudo cp %s %s", filepath.Dir(kubeProxyConfig.Kubeconfig), remotePath, kubeProxyConfig.Kubeconfig)
	_, err = ss.Run(
		copyCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", copyCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	catCmd := fmt.Sprintf("sudo cat %s", kubeProxyConfig.Kubeconfig)
	var out []byte
	out, err = ss.Run(
		catCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil || len(out) == 0 {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", catCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	fmt.Println("kube-proxy KUBECONFIG:", kubeProxyConfig.Kubeconfig, string(out))
	return nil
}

func writeKubeProxyEnvFile(target ec2config.Instance, kubeProxyConfig kubernetesconfig.KubeProxy) (p string, err error) {
	kubeProxyConfig.HostnameOverride = target.PrivateDNSName
	var sc string
	sc, err = kubeProxyConfig.Sysconfig()
	if err != nil {
		return "", fmt.Errorf("failed to create kube-proxy sysconfig (%v)", err)
	}
	p, err = fileutil.WriteTempFile([]byte(sc))
	if err != nil {
		return "", fmt.Errorf("failed to write kube-proxy sysconfig file (%v)", err)
	}
	return p, nil
}

func sendKubeProxyEnvFile(
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

	remotePath := fmt.Sprintf("/home/%s/kube-proxy.sysconfig", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	copyCmd := fmt.Sprintf("sudo mkdir -p /etc/sysconfig/ && sudo cp %s /etc/sysconfig/kube-proxy", remotePath)
	_, err = ss.Run(
		copyCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", copyCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	catCmd := "sudo cat /etc/sysconfig/kube-proxy"
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

func writeKubeProxyServiceFile(kubeProxyConfig kubernetesconfig.KubeProxy) (p string, err error) {
	var sc string
	sc, err = kubeProxyConfig.Service()
	if err != nil {
		return "", fmt.Errorf("failed to create kube-proxy service file (%v)", err)
	}
	p, err = fileutil.WriteTempFile([]byte(sc))
	if err != nil {
		return "", fmt.Errorf("failed to write kube-proxy service file (%v)", err)
	}
	return p, nil
}

func sendKubeProxyServiceFile(
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

	remotePath := fmt.Sprintf("/home/%s/kube-proxy.install.sh", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	remoteCmd := fmt.Sprintf("chmod +x %s && sudo bash %s", remotePath, remotePath)
	_, err = ss.Run(
		remoteCmd,
		ssh.WithTimeout(30*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to execute %q for %q(%q) (error %v)", remoteCmd, ec2Config.ClusterName, target.InstanceID, err)
	}
	return nil
}
