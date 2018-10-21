package eks

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/awstester/internal/ssh"
	"github.com/aws/awstester/pkg/fileutil"

	"go.uber.org/zap"
)

// https://github.com/kubernetes/test-infra/blob/master/kubetest/dump.go
func (md *embedded) downloadWorkerNodeLogs() (err error) {
	if !md.cfg.EnableNodeSSH {
		return errors.New("node SSH is not enabled")
	}

	paths := make(map[string]string)

	md.ec2InstancesMu.RLock()
	defer md.ec2InstancesMu.RUnlock()

	for _, iv := range md.ec2Instances {
		id, ip := *iv.InstanceId, *iv.PublicIpAddress
		pfx := strings.TrimSpace(fmt.Sprintf("%s-%s", id, ip))

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
		var cmd string

		// https://github.com/awslabs/amazon-eks-ami/blob/master/files/logrotate-kube-proxy
		md.lg.Info(
			"fetching kube-proxy logs",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
		)
		cmd = "cat /var/log/kube-proxy.log"
		out, err = sh.Run(cmd)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to run command",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("cmd", cmd),
				zap.Error(err),
			)
			return err
		}
		fpath, err = fileutil.WriteToTempDir(pfx+".kube-proxy.log", out)
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
		paths[fpath] = filepath.Base(fpath)

		// kernel logs
		md.lg.Info(
			"fetching kernel logs",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
		)
		cmd = "sudo journalctl --output=short-precise -k"
		out, err = sh.Run(cmd)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to run command",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("cmd", cmd),
				zap.Error(err),
			)
			return err
		}
		fpath, err = fileutil.WriteToTempDir(pfx+".kernel.log", out)
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
		paths[fpath] = filepath.Base(fpath)

		// full journal logs (e.g. disk mounts)
		md.lg.Info(
			"fetching journal logs",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
		)
		cmd = "sudo journalctl --output=short-precise"
		out, err = sh.Run(cmd)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to run command",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("cmd", cmd),
				zap.Error(err),
			)
			return err
		}
		fpath, err = fileutil.WriteToTempDir(pfx+".journal.log", out)
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
		paths[fpath] = filepath.Base(fpath)

		// other systemd services
		md.lg.Info(
			"fetching all systemd services",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
		)
		cmd = "sudo journalctl --output=short-precise"
		out, err = sh.Run(cmd)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to run command",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("cmd", cmd),
				zap.Error(err),
			)
			return err
		}
		var svcs []string
		for _, line := range strings.Split(string(out), "\n") {
			tokens := strings.Fields(line)
			if len(tokens) == 0 || tokens[0] == "" {
				continue
			}
			svcs = append(svcs, tokens[0])
		}
		for _, svc := range svcs {
			name := svc + ".service"
			md.lg.Info(
				"fetching systemd service log",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("name", name),
			)
			cmd = "sudo journalctl --output=cat -u " + name
			out, err = sh.Run(cmd)
			if err != nil {
				sh.Close()
				md.lg.Warn(
					"failed to run command",
					zap.String("instance-id", id),
					zap.String("public-ip", ip),
					zap.String("cmd", cmd),
					zap.Error(err),
				)
				return err
			}
			fpath, err = fileutil.WriteToTempDir(pfx+"."+name+".log", out)
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
			paths[fpath] = filepath.Base(fpath)
		}

		// other /var/log
		md.lg.Info(
			"fetching all /var/log",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
		)
		cmd = "sudo find /var/log -print0"
		out, err = sh.Run(cmd)
		if err != nil {
			sh.Close()
			md.lg.Warn(
				"failed to run command",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("cmd", cmd),
				zap.Error(err),
			)
			return err
		}
		var logPaths []string
		for _, v := range bytes.Split(out, []byte{0}) {
			if len(v) == 0 {
				// last value
				continue
			}
			logPaths = append(logPaths, string(v))
		}
		for _, p := range logPaths {
			md.lg.Info(
				"fetching /var/log",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("path", p),
			)
			cmd = "sudo cat" + p
			out, err = sh.Run(cmd)
			if err != nil {
				sh.Close()
				md.lg.Warn(
					"failed to run command",
					zap.String("cmd", cmd),
					zap.String("instance-id", id),
					zap.String("public-ip", ip),
					zap.Error(err),
				)
				return err
			}
			fpath, err = fileutil.WriteToTempDir(pfx+"."+p, out)
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
			paths[fpath] = filepath.Base(fpath)
		}

		sh.Close()
	}

	md.cfg.SetWorkerNodeLogs(paths)
	return nil
}

/*
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
fpath, err = fileutil.WriteToTempDir( pfx + "." + p,out)
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
paths[fpath] = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%s-%s.kubelet.log", id, ip)))
*/
