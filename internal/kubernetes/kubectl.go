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

func writeKubectlKubeConfigFile(
	privateKey []byte,
	publicKey []byte,
	rootCA []byte,
	clusterName string,
	loadBalancerURL string,
) (p string, err error) {
	// TODO
	name := clusterName + ".k8s.local"
	cfg := clientcmdapi.NewConfig()
	cfg.APIVersion = "v1"
	cfg.Kind = "Config"
	cfg.Clusters[name] = &clientcmdapi.Cluster{
		CertificateAuthorityData: rootCA,
		Server:                   loadBalancerURL,
	}
	cfg.Contexts[name] = &clientcmdapi.Context{
		Cluster:  name,
		AuthInfo: name,
	}
	cfg.CurrentContext = name
	// TODO: enable basic auth?
	cfg.AuthInfos[name] = &clientcmdapi.AuthInfo{
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
		return "", fmt.Errorf("failed to write kubelet KUBECONFIG file (%v)", err)
	}
	return p, nil
}

func sendKubectlKubeConfigFile(
	lg *zap.Logger,
	ec2Config ec2config.Config,
	target ec2config.Instance,
	filePathToSend string,
	kubectlConfig kubernetesconfig.Kubectl,
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

	remotePath := fmt.Sprintf("/home/%s/kubectl.kubeconfig", ec2Config.UserName)
	_, err = ss.Send(
		filePathToSend,
		remotePath,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send %q to %q for %q(%q) (error %v)", filePathToSend, remotePath, ec2Config.ClusterName, target.InstanceID, err)
	}

	copyCmd := fmt.Sprintf("sudo mkdir -p %s && sudo cp %s %s", filepath.Dir(kubectlConfig.Kubeconfig), remotePath, kubectlConfig.Kubeconfig)
	_, err = ss.Run(
		copyCmd,
		ssh.WithTimeout(15*time.Second),
		ssh.WithRetry(3, 3*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to %q for %q(%q) (error %v)", copyCmd, ec2Config.ClusterName, target.InstanceID, err)
	}

	catCmd := fmt.Sprintf("sudo chmod 0666 %s && sudo cat %s", kubectlConfig.Kubeconfig, kubectlConfig.Kubeconfig)
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
