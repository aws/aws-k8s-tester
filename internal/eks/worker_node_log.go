package eks

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/awstester/internal/ssh"
	"github.com/aws/awstester/pkg/fileutil"

	"go.uber.org/zap"
)

// https://github.com/kubernetes/test-infra/blob/master/kubetest/dump.go
func (md *embedded) getWorkerNodeLogs() (err error) {
	if !md.cfg.EnableNodeSSH {
		return errors.New("node SSH is not enabled")
	}

	paths := make(map[string]string)

	md.ec2Mu.Lock()
	defer md.ec2Mu.Unlock()

	for _, iv := range md.ec2Instances {
		id, ip := *iv.InstanceId, *iv.PublicIpAddress

		var sh ssh.SSH
		sh, err = ssh.New(ssh.Config{
			Logger:   md.lg,
			KeyPath:  md.cfg.ClusterState.CFStackWorkerNodeGroupKeyPairPrivateKeyPath,
			Addr:     ip + ":22",
			UserName: "ubuntu",
		})
		if err != nil {
			md.lg.Warn(
				"failed to create SSH",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return err
		}
		if err = sh.Connect(); err != nil {
			md.lg.Warn(
				"failed to connect",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return err
		}

		var out []byte
		var fpath string

		// https://github.com/awslabs/amazon-eks-ami/blob/master/files/logrotate-kube-proxy
		md.lg.Info(
			"fetching kube-proxy logs",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
		)
		kubeProxyCmd := "cat /var/log/kube-proxy.log"
		out, err = sh.Run(kubeProxyCmd)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to run command",
				zap.String("cmd", kubeProxyCmd),
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return err
		}
		fpath, err = fileutil.WriteTempFile(out)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to write output",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return err
		}
		paths[fpath] = strings.ToLower(strings.TrimSpace(fmt.Sprintf("kube-proxy-%s-%s.log", id, ip)))

		// https://github.com/awslabs/amazon-eks-ami/blob/master/files/kubelet.service
		md.lg.Info(
			"fetching kubelet.service logs",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
		)
		kubeletCmd := ""
		out, err = sh.Run(kubeletCmd)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to run command",
				zap.String("cmd", kubeletCmd),
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return err
		}
		fpath, err = fileutil.WriteTempFile(out)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to write output",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return err
		}
		paths[fpath] = strings.ToLower(strings.TrimSpace(fmt.Sprintf("kubelet-%s-%s.log", id, ip)))

		// kernel logs
		// TODO

		// full journal logs (e.g. disk mounts)
		// TODO

		// other systemd services
		// TODO

		// other /var/log
		// TODO

		sh.Close()
	}

	md.ec2Logs = paths
	return nil
}
