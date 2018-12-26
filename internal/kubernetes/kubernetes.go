// Package kubernetes implements Kubernetes test operations.
package kubernetes

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Deployer defines kubernetes test operation.
type Deployer interface {
	Create() error
	Terminate() error
}

type embedded struct {
	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *kubernetesconfig.Config

	ec2MasterNodesDeployer ec2.Deployer
	ec2WorkerNodesDeployer ec2.Deployer
}

// NewDeployer creates a new embedded kubernetes tester.
func NewDeployer(cfg *kubernetesconfig.Config) (Deployer, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}
	lg, err := zaputil.New(cfg.LogDebug, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}
	md := &embedded{lg: lg, cfg: cfg}
	md.ec2MasterNodesDeployer, err = ec2.NewDeployer(md.cfg.EC2MasterNodes)
	if err != nil {
		return nil, err
	}
	md.ec2WorkerNodesDeployer, err = ec2.NewDeployer(md.cfg.EC2WorkerNodes)
	if err != nil {
		return nil, err
	}
	return md, cfg.Sync()
}

func (md *embedded) Create() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	now := time.Now().UTC()

	md.cfg.ConfigPathURL = genS3URL(md.cfg.EC2MasterNodes.AWSRegion, md.cfg.Tag, md.cfg.EC2MasterNodes.ConfigPathBucket)
	md.cfg.KubeConfigPathURL = genS3URL(md.cfg.EC2MasterNodes.AWSRegion, md.cfg.Tag, md.cfg.KubeConfigPathBucket)
	md.cfg.LogOutputToUploadPathURL = genS3URL(md.cfg.EC2MasterNodes.AWSRegion, md.cfg.Tag, md.cfg.EC2MasterNodes.LogOutputToUploadPathBucket)

	if err = md.ec2MasterNodesDeployer.Create(); err != nil {
		return err
	}
	md.cfg.Sync()
	if err = md.ec2WorkerNodesDeployer.Create(); err != nil {
		return err
	}
	md.cfg.Sync()

	md.lg.Info(
		"deployed EC2",
		zap.Strings("plugins-master-nodes", md.cfg.EC2MasterNodes.Plugins),
		zap.String("vpc-id-master-nodes", md.cfg.EC2MasterNodes.VPCID),
		zap.Strings("plugins-worker-nodes", md.cfg.EC2WorkerNodes.Plugins),
		zap.String("vpc-id-worker-nodes", md.cfg.EC2WorkerNodes.VPCID),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	if md.cfg.LogDebug {
		fmt.Println("EC2MasterNodes.SSHCommands:", md.cfg.EC2MasterNodes.SSHCommands())
		fmt.Println("EC2WorkerNodes.SSHCommands:", md.cfg.EC2WorkerNodes.SSHCommands())
	}
	if err = md.cfg.ValidateAndSetDefaults(); err != nil {
		return err
	}

	// SCP to each EC2MasterNodes instance
	md.lg.Info("deploying kubernetes master nodes")
	var masterEC2 ec2config.Instance
	for _, iv := range md.cfg.EC2MasterNodes.Instances {
		masterEC2 = iv
		break
	}
	var masterSSH ssh.SSH
	masterSSH, err = ssh.New(ssh.Config{
		Logger:        md.lg,
		KeyPath:       md.cfg.EC2MasterNodes.KeyPath,
		PublicIP:      masterEC2.PublicIP,
		PublicDNSName: masterEC2.PublicDNSName,
		UserName:      md.cfg.EC2MasterNodes.UserName,
	})
	if err != nil {
		return err
	}
	if err = masterSSH.Connect(); err != nil {
		return err
	}
	defer masterSSH.Close()
	var out []byte
	out, err = masterSSH.Run(
		"ls /usr/bin",
		ssh.WithTimeout(15*time.Second),
	)
	fmt.Println("masterSSH /usr/bin:", string(out))

	md.lg.Info("deploying kubernetes worker nodes")
	var workerEC2 ec2config.Instance
	for _, iv := range md.cfg.EC2WorkerNodes.Instances {
		workerEC2 = iv
		break
	}
	var workerSSH ssh.SSH
	workerSSH, err = ssh.New(ssh.Config{
		Logger:        md.lg,
		KeyPath:       md.cfg.EC2WorkerNodes.KeyPath,
		PublicIP:      workerEC2.PublicIP,
		PublicDNSName: workerEC2.PublicDNSName,
		UserName:      md.cfg.EC2WorkerNodes.UserName,
	})
	if err != nil {
		return err
	}
	if err = workerSSH.Connect(); err != nil {
		return err
	}
	defer workerSSH.Close()
	out, err = workerSSH.Run(
		"ls /usr/bin",
		ssh.WithTimeout(15*time.Second),
	)
	fmt.Println("workerSSH /usr/bin:", string(out))

	return md.cfg.Sync()
}

func (md *embedded) Terminate() error {
	md.mu.Lock()
	defer md.mu.Unlock()

	md.lg.Info("terminating kubernetes")
	if md.cfg.UploadTesterLogs && len(md.cfg.EC2MasterNodes.Instances) > 0 {
		err := md.ec2MasterNodesDeployer.UploadToBucketForTests(md.cfg.KubeConfigPath, md.cfg.KubeConfigPathBucket)
		md.lg.Info("uploaded KUBECONFIG", zap.Error(err))

		var fpathToS3Path map[string]string
		fpathToS3Path, err = fetchLogs(
			md.lg,
			md.cfg.EC2MasterNodes.UserName,
			md.cfg.ClusterName,
			md.cfg.EC2MasterNodes.KeyPath,
			md.cfg.EC2MasterNodes.Instances,
		)
		md.cfg.Logs = fpathToS3Path
		err = md.uploadLogs()
		md.lg.Info("uploaded", zap.Error(err))
	}

	return md.ec2MasterNodesDeployer.Terminate()
}

func (md *embedded) uploadLogs() (err error) {
	ess := []string{}
	for k, v := range md.cfg.Logs {
		err = md.ec2MasterNodesDeployer.UploadToBucketForTests(k, v)
		md.lg.Info("uploaded kubernetes log", zap.Error(err))
		if err != nil {
			ess = append(ess, err.Error())
		}
	}
	return errors.New(strings.Join(ess, ", "))
}

// TODO: parallelize
func fetchLogs(
	lg *zap.Logger,
	userName string,
	clusterName string,
	privateKeyPath string,
	nodes map[string]ec2config.Instance) (fpathToS3Path map[string]string, err error) {
	fpathToS3Path = make(map[string]string)
	for _, iv := range nodes {
		var fm map[string]string
		fm, err = fetchLog(lg, userName, clusterName, privateKeyPath, iv)
		if err != nil {
			return nil, err
		}
		for k, v := range fm {
			fpathToS3Path[k] = v
		}
	}
	return fpathToS3Path, nil
}

func fetchLog(
	lg *zap.Logger,
	userName string,
	clusterName string,
	privateKeyPath string,
	inst ec2config.Instance) (fpathToS3Path map[string]string, err error) {
	id, ip := inst.InstanceID, inst.PublicIP

	var sh ssh.SSH
	sh, err = ssh.New(ssh.Config{
		Logger:        lg,
		KeyPath:       privateKeyPath,
		UserName:      userName,
		PublicIP:      inst.PublicIP,
		PublicDNSName: inst.PublicDNSName,
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
	defer sh.Close()

	var out []byte
	out, err = sh.Run(
		"sudo journalctl --no-pager -u kubelet.service",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return nil, err
	}
	var kubeletLogPath string
	kubeletLogPath, err = fileutil.WriteTempFile(out)
	if err != nil {
		return nil, err
	}

	lg.Info("downloaded kubernetes log", zap.String("path", kubeletLogPath))
	fpathToS3Path = make(map[string]string)
	fpathToS3Path[kubeletLogPath] = fmt.Sprintf("%s/%s-kubelet.log", clusterName, id)
	return fpathToS3Path, nil
}

// genS3URL returns S3 URL path.
// e.g. https://s3-us-west-2.amazonaws.com/aws-k8s-tester-20180925/hello-world
func genS3URL(region, bucket, s3Path string) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", region, bucket, s3Path)
}
