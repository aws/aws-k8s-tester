package eks

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"

	"go.uber.org/zap"
)

func fetchWorkerNodeLog(
	lg *zap.Logger,
	userName string,
	clusterName string,
	privateKeyPath string,
	workerNode ec2config.Instance) (fpathToS3Path map[string]string, err error) {
	fpathToS3Path = make(map[string]string)

	id, ip := workerNode.InstanceID, workerNode.PublicIP
	pfx := strings.TrimSpace(fmt.Sprintf("%s-%s", id, ip))

	var sh ssh.SSH
	sh, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       privateKeyPath,
		PublicIP:      ip,
		PublicDNSName: workerNode.PublicDNSName,
		UserName:      userName,
	})
	if err != nil {
		lg.Warn(
			"failed to create SSH",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.Error(err),
		)
		return nil, err
	}

	if err = sh.Connect(); err != nil {
		lg.Warn(
			"failed to connect",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.Error(err),
		)
		return nil, err
	}

	var out []byte
	var fpath string
	var cmd string

	// https://github.com/awslabs/amazon-eks-ami/blob/master/files/logrotate-kube-proxy
	lg.Info(
		"fetching kube-proxy logs",
		zap.String("instance-id", id),
		zap.String("public-ip", ip),
	)
	cmd = "cat /var/log/kube-proxy.log"
	out, err = sh.Run(cmd)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to run command",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.String("cmd", cmd),
			zap.Error(err),
		)
		return nil, err
	}
	fpath, err = fileutil.WriteToTempDir(pfx+".kube-proxy.log", out)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to write output",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.Error(err),
		)
		return nil, err
	}
	fpathToS3Path[fpath] = filepath.Join(clusterName, pfx, filepath.Base(fpath))

	// kernel logs
	lg.Info(
		"fetching kernel logs",
		zap.String("instance-id", id),
		zap.String("public-ip", ip),
	)
	cmd = "sudo journalctl --no-pager --output=short-precise -k"
	out, err = sh.Run(cmd)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to run command",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.String("cmd", cmd),
			zap.Error(err),
		)
		return nil, err
	}
	fpath, err = fileutil.WriteToTempDir(pfx+".kernel.log", out)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to write output",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.Error(err),
		)
		return nil, err
	}
	fpathToS3Path[fpath] = filepath.Join(clusterName, pfx, filepath.Base(fpath))

	// full journal logs (e.g. disk mounts)
	lg.Info(
		"fetching journal logs",
		zap.String("instance-id", id),
		zap.String("public-ip", ip),
	)
	cmd = "sudo journalctl --no-pager --output=short-precise"
	out, err = sh.Run(cmd)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to run command",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.String("cmd", cmd),
			zap.Error(err),
		)
		return nil, err
	}
	fpath, err = fileutil.WriteToTempDir(pfx+".journal.log", out)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to write output",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.Error(err),
		)
		return nil, err
	}
	fpathToS3Path[fpath] = filepath.Join(clusterName, pfx, filepath.Base(fpath))

	// other systemd services
	lg.Info(
		"fetching all systemd services",
		zap.String("instance-id", id),
		zap.String("public-ip", ip),
	)
	cmd = "sudo systemctl list-units -t service --no-pager --no-legend --all"
	out, err = sh.Run(cmd)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to run command",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.String("cmd", cmd),
			zap.Error(err),
		)
		return nil, err
	}

	/*
		auditd.service                                        loaded    active   running Security Auditing Service
		auth-rpcgss-module.service                            loaded    inactive dead    Kernel Module supporting RPCSEC_GSS
	*/
	var svcs []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "" || len(fields) < 5 {
			continue
		}
		if fields[1] == "not-found" {
			continue
		}
		if fields[2] == "inactive" {
			continue
		}
		svcs = append(svcs, fields[0])
	}
	for _, svc := range svcs {
		lg.Info(
			"fetching systemd service log",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.String("service", svc),
		)
		cmd = "sudo journalctl --no-pager --output=cat -u " + svc
		out, err = sh.Run(cmd)
		if err != nil {
			lg.Warn(
				"failed to run command",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.String("cmd", cmd),
				zap.Error(err),
			)
			continue
		}
		if len(out) == 0 {
			lg.Info("empty log", zap.String("service", svc))
			continue
		}
		fpath, err = fileutil.WriteToTempDir(pfx+"."+svc+".log", out)
		if err != nil {
			sh.Close()
			lg.Warn(
				"failed to write output",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return nil, err
		}
		fpathToS3Path[fpath] = filepath.Join(clusterName, pfx, filepath.Base(fpath))
	}

	// other /var/log
	lg.Info(
		"fetching all /var/log",
		zap.String("instance-id", id),
		zap.String("public-ip", ip),
	)
	cmd = "sudo find /var/log ! -type d"
	out, err = sh.Run(cmd)
	if err != nil {
		sh.Close()
		lg.Warn(
			"failed to run command",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.String("cmd", cmd),
			zap.Error(err),
		)
		return nil, err
	}
	var varLogs []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) == 0 {
			// last value
			continue
		}
		varLogs = append(varLogs, line)
	}
	for _, p := range varLogs {
		lg.Info(
			"fetching /var/log",
			zap.String("instance-id", id),
			zap.String("public-ip", ip),
			zap.String("path", p),
		)
		cmd = "sudo cat " + p
		out, err = sh.Run(cmd)
		if err != nil {
			lg.Warn(
				"failed to run command",
				zap.String("cmd", cmd),
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			continue
		}
		if len(out) == 0 {
			lg.Info("empty log", zap.String("path", p))
			continue
		}
		fpath, err = fileutil.WriteToTempDir(pfx+strings.Replace(p, "/", ".", -1), out)
		if err != nil {
			sh.Close()
			lg.Warn(
				"failed to write output",
				zap.String("instance-id", id),
				zap.String("public-ip", ip),
				zap.Error(err),
			)
			return nil, err
		}
		fpathToS3Path[fpath] = filepath.Join(clusterName, pfx, filepath.Base(fpath))
	}

	sh.Close()

	return fpathToS3Path, nil
}

func fetchWorkerNodeLogs(
	lg *zap.Logger,
	userName string,
	clusterName string,
	privateKeyPath string,
	workerNodes map[string]ec2config.Instance) (fpathToS3Path map[string]string, err error) {

	// create new channel for multiple goroutines. and a tokens channel to limit parallelism
	c := make(chan map[string]string)
	tokens := make(chan string, 200)

	// loop through nodes and send goroutine
	for _, iv := range workerNodes {
		go concurrentFetchLog(lg, userName, clusterName, privateKeyPath, iv, c, tokens)
	}

	// create new map fpathToS3Path to join all the data
	fpathToS3Path = make(map[string]string)

	// join data from each goroutine and report to new channel when done
	done := make(chan bool, 1)
	go joinData(c, done, fpathToS3Path, len(workerNodes))
	<-done

	// return map of all data collected
	return fpathToS3Path, nil
}

func concurrentFetchLog(
	lg *zap.Logger,
	userName string,
	clusterName string,
	privateKeyPath string,
	workerNode ec2config.Instance,
	channel chan map[string]string,
	tokens chan string) {

	// push something into tokens channel, to signal that resources are now being used.
	tokens <- "token"

	// send request to fetchWorkerLog
	fm, err := fetchWorkerNodeLog(lg, userName, clusterName, privateKeyPath, workerNode)
	if err != nil {
		// TODO: Handle error
	}

	// take back token from channel, to signal that we're done using resouces.
	<-tokens
	// send map received to channel
	channel <- fm

}

func joinData(
	channel chan map[string]string,
	done chan<- bool,
	joinedData map[string]string,
	desired int) {

	for i := 0; i < desired; i++ {
		dataSubset := <-channel
		for k, v := range dataSubset {
			joinedData[k] = v
		}
	}

	done <- true
}
