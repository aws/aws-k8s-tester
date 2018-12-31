// Package kubernetes implements Kubernetes operations.
package kubernetes

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/etcd"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubernetesconfig"
	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"
	"github.com/aws/aws-k8s-tester/storagetester"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
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

	etcdTester             storagetester.Tester
	ec2MasterNodesDeployer ec2.Deployer
	ec2WorkerNodesDeployer ec2.Deployer

	ss    *session.Session
	elbv1 elbiface.ELBAPI     // for classic ELB
	elbv2 elbv2iface.ELBV2API // for ALB or NLB
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

	awsCfg := &awsapi.Config{
		Logger:         md.lg,
		DebugAPICalls:  cfg.LogDebug,
		Region:         cfg.AWSRegion,
		CustomEndpoint: "",
	}
	md.ss, err = awsapi.New(awsCfg)
	if err != nil {
		return nil, err
	}
	md.elbv1 = elb.New(md.ss)
	md.elbv2 = elbv2.New(md.ss)

	md.etcdTester, err = etcd.NewTester(md.cfg.ETCDNodes)
	if err != nil {
		return nil, err
	}
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

	defer func() {
		if err != nil {
			md.lg.Warn("failed to create Kubernetes, reverting", zap.Error(err))
			md.lg.Warn("failed to create Kubernetes, reverted", zap.Error(md.terminate()))
		}
	}()

	// shared master node VPC and subnets for: etcd nodes, worker nodes
	// do not run this in goroutine, since VPC for master nodes have to be created at first
	if err = md.ec2MasterNodesDeployer.Create(); err != nil {
		return err
	}
	md.cfg.EC2MasterNodesCreated = true
	md.cfg.Sync()

	md.cfg.ETCDNodes.EC2.VPCID = md.cfg.EC2MasterNodes.VPCID
	md.cfg.ETCDNodes.EC2Bastion.VPCID = md.cfg.EC2MasterNodes.VPCID
	md.cfg.EC2WorkerNodes.VPCID = md.cfg.EC2MasterNodes.VPCID

	// prevent VPC double-delete
	md.cfg.ETCDNodes.EC2.VPCCreated = false
	md.cfg.ETCDNodes.EC2Bastion.VPCCreated = false
	md.cfg.EC2WorkerNodes.VPCCreated = false
	md.cfg.Sync()

	errc, ess := make(chan error), make([]string, 0)
	var mu sync.Mutex
	go func() {
		if err = md.etcdTester.Create(); err != nil {
			errc <- fmt.Errorf("failed to create etcd cluster (%v)", err)
			return
		}
		mu.Lock()
		md.cfg.ETCDNodesCreated = true
		mu.Unlock()
		md.cfg.Sync()
		errc <- nil
	}()
	go func() {
		if err = md.ec2WorkerNodesDeployer.Create(); err != nil {
			errc <- fmt.Errorf("failed to create worker nodes (%v)", err)
			return
		}
		mu.Lock()
		md.cfg.EC2WorkerNodesCreated = true
		mu.Unlock()
		md.cfg.Sync()
		errc <- nil
	}()
	for range []int{0, 1} {
		if err = <-errc; err != nil {
			ess = append(ess, err.Error())
		}
	}
	if len(ess) > 0 {
		return errors.New(strings.Join(ess, ", "))
	}
	md.lg.Info(
		"deployed EC2 instances",
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

	var elbOut *elb.CreateLoadBalancerOutput
	elbOut, err = md.elbv1.CreateLoadBalancer(&elb.CreateLoadBalancerInput{
		LoadBalancerName: aws.String(md.cfg.LoadBalancerName),
		SecurityGroups:   aws.StringSlice(md.cfg.EC2MasterNodes.SecurityGroupIDs),
		Subnets:          aws.StringSlice(md.cfg.EC2MasterNodes.SubnetIDs),
		Listeners: []*elb.Listener{
			{
				InstancePort:     aws.Int64(443),
				InstanceProtocol: aws.String("TCP"),
				LoadBalancerPort: aws.Int64(443),
				Protocol:         aws.String("TCP"),
			},
		},
		Tags: []*elb.Tag{
			{Key: aws.String("Name"), Value: aws.String(md.cfg.LoadBalancerName)},
		},
	})
	if err != nil {
		return err
	}
	md.cfg.LoadBalancerCreated = true
	md.cfg.Sync()
	md.lg.Info("created load balancer", zap.String("name", md.cfg.LoadBalancerName), zap.String("dns-name", *elbOut.DNSName))

	instances := make([]*elb.Instance, 0, len(md.cfg.EC2MasterNodes.Instances)+len(md.cfg.EC2WorkerNodes.Instances))
	for _, iv := range md.cfg.EC2MasterNodes.Instances {
		instances = append(instances, &elb.Instance{
			InstanceId: aws.String(iv.InstanceID),
		})
	}
	for _, iv := range md.cfg.EC2WorkerNodes.Instances {
		instances = append(instances, &elb.Instance{
			InstanceId: aws.String(iv.InstanceID),
		})
	}
	if _, err = md.elbv1.RegisterInstancesWithLoadBalancer(&elb.RegisterInstancesWithLoadBalancerInput{
		LoadBalancerName: aws.String(md.cfg.LoadBalancerName),
		Instances:        instances,
	}); err != nil {
		return err
	}
	md.cfg.LoadBalancerRegistered = true
	md.cfg.Sync()
	md.lg.Info("registered instances to load balancer", zap.String("name", md.cfg.LoadBalancerName), zap.Int("instances", len(instances)))

	downloadsMaster := md.cfg.DownloadsMaster()
	downloadsWorker := md.cfg.DownloadsWorker()
	for _, masterEC2 := range md.cfg.EC2MasterNodes.Instances {
		go func(inst ec2config.Instance) {
			instSSH, serr := ssh.New(ssh.Config{
				Logger:        md.lg,
				KeyPath:       md.cfg.EC2MasterNodes.KeyPath,
				PublicIP:      inst.PublicIP,
				PublicDNSName: inst.PublicDNSName,
				UserName:      md.cfg.EC2MasterNodes.UserName,
			})
			if serr != nil {
				errc <- fmt.Errorf("failed to create a SSH to master node %q(%q) (error %v)", inst.InstanceID, inst.PublicIP, serr)
				return
			}
			if serr = instSSH.Connect(); serr != nil {
				errc <- fmt.Errorf("failed to connect to master node %q(%q) (error %v)", inst.InstanceID, inst.PublicIP, serr)
				return
			}
			defer instSSH.Close()

			for _, v := range downloadsMaster {
				out, oerr := instSSH.Run(
					v.DownloadCommand,
					ssh.WithTimeout(15*time.Second),
					ssh.WithRetry(3, 3*time.Second),
				)
				if oerr != nil {
					errc <- fmt.Errorf("failed %q to master node %q(%q) (error %v)", v.DownloadCommand, inst.InstanceID, inst.PublicIP, oerr)
					return
				}
				md.lg.Info(
					"installed at master node",
					zap.String("instance-id", inst.InstanceID),
					zap.String("path", v.Path),
					zap.String("output", string(out)),
				)
			}

			// TODO: now that binaries are installed, now set up service file

			errc <- nil
		}(masterEC2)
	}
	for range md.cfg.EC2MasterNodes.Instances {
		err = <-errc
		if err != nil {
			ess = append(ess, err.Error())
		}
	}
	if len(ess) > 0 {
		return errors.New(strings.Join(ess, ", "))
	}
	md.lg.Info("deployed kubernetes master nodes")

	for _, workerEC2 := range md.cfg.EC2WorkerNodes.Instances {
		go func(inst ec2config.Instance) {
			instSSH, serr := ssh.New(ssh.Config{
				Logger:        md.lg,
				KeyPath:       md.cfg.EC2WorkerNodes.KeyPath,
				PublicIP:      inst.PublicIP,
				PublicDNSName: inst.PublicDNSName,
				UserName:      md.cfg.EC2WorkerNodes.UserName,
			})
			if serr != nil {
				errc <- fmt.Errorf("failed to create a SSH to worker node %q(%q) (error %v)", inst.InstanceID, inst.PublicIP, serr)
				return
			}
			if serr = instSSH.Connect(); serr != nil {
				errc <- fmt.Errorf("failed to connect to worker node %q(%q) (error %v)", inst.InstanceID, inst.PublicIP, serr)
				return
			}
			defer instSSH.Close()

			for _, v := range downloadsWorker {
				out, oerr := instSSH.Run(
					v.DownloadCommand,
					ssh.WithTimeout(15*time.Second),
					ssh.WithRetry(3, 3*time.Second),
				)
				if oerr != nil {
					errc <- fmt.Errorf("failed %q to worker node %q(%q) (error %v)", v.DownloadCommand, inst.InstanceID, inst.PublicIP, oerr)
					return
				}
				md.lg.Info(
					"installed at worker node",
					zap.String("instance-id", inst.InstanceID),
					zap.String("path", v.Path),
					zap.String("output", string(out)),
				)
			}

			// TODO

			errc <- nil
		}(workerEC2)
	}
	for range md.cfg.EC2WorkerNodes.Instances {
		err = <-errc
		if err != nil {
			ess = append(ess, err.Error())
		}
	}
	if len(ess) > 0 {
		return errors.New(strings.Join(ess, ", "))
	}
	md.lg.Info("deployed kubernetes worker nodes")

	if md.cfg.UploadKubeConfig {
		err := md.ec2MasterNodesDeployer.UploadToBucketForTests(md.cfg.KubeConfigPath, md.cfg.KubeConfigPathBucket)
		if err == nil {
			md.lg.Info("uploaded KUBECONFIG", zap.String("path", md.cfg.KubeConfigPath))
		} else {
			md.lg.Warn("failed to upload KUBECONFIG", zap.String("path", md.cfg.KubeConfigPath), zap.Error(err))
		}
	}

	return md.cfg.Sync()
}

/*
# keep in sync with
# https://github.com/kubernetes/kubernetes/blob/master/build/debs/kubelet.service
cat <<EOF > /tmp/kubelet.service
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=http://kubernetes.io/docs/

[Service]
ExecStart=/usr/bin/kubelet
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
cat /tmp/kubelet.service

sudo mkdir -p /etc/systemd/system/kubelet.service.d
sudo cp /tmp/kubelet.service /etc/systemd/system/kubelet.service

sudo systemctl daemon-reload
sudo systemctl cat kubelet.service


// CreateInstall creates Kubernetes install script.
func CreateInstall(ver string) (string, error) {
	tpl := template.Must(template.New("installKubernetesAmazonLinux2Template").Parse(installKubernetesAmazonLinux2Template))
	buf := bytes.NewBuffer(nil)
	kv := kubernetesInfo{Version: ver}
	if err := tpl.Execute(buf, kv); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type kubernetesInfo struct {
	Version string
}

RELEASE=v{{ .Version }}

cd /usr/bin
sudo rm -f /usr/bin/{kube-proxy,kubectl,kubelet,kube-apiserver,kube-controller-manager,kube-scheduler,cloud-controller-manager}

sudo curl --silent -L --remote-name-all https://storage.googleapis.com/kubernetes-release/release/v1.13.0/bin/linux/amd64/{kube-proxy,kubectl,kubelet,kube-apiserver,kube-controller-manager,kube-scheduler,cloud-controller-manager}
sudo chmod +x {kube-proxy,kubectl,kubelet,kube-apiserver,kube-controller-manager,kube-scheduler,cloud-controller-manager}

https://github.com/kubernetes/kubernetes/blob/master/build/debs/kubelet.service

sudo systemctl enable kubelet && sudo systemctl restart kubelet
sudo systemctl status kubelet --full --no-pager || true
sudo journalctl --no-pager --output=cat -u kubelet
*/

func (md *embedded) Terminate() error {
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.terminate()
}

func (md *embedded) terminate() error {
	if md.cfg.UploadKubeConfig && md.cfg.EC2MasterNodesCreated {
		err := md.ec2MasterNodesDeployer.UploadToBucketForTests(md.cfg.KubeConfigPath, md.cfg.KubeConfigPathBucket)
		if err == nil {
			md.lg.Info("uploaded KUBECONFIG", zap.String("path", md.cfg.KubeConfigPath))
		} else {
			md.lg.Warn("failed to upload KUBECONFIG", zap.String("path", md.cfg.KubeConfigPath), zap.Error(err))
		}
	}

	md.lg.Info("terminating kubernetes")
	if md.cfg.UploadTesterLogs && len(md.cfg.EC2MasterNodes.Instances) > 0 && md.cfg.EC2MasterNodesCreated {
		fpathToS3PathMasterNodes, err := fetchLogs(
			md.lg,
			md.cfg.EC2MasterNodes.UserName,
			md.cfg.ClusterName,
			md.cfg.EC2MasterNodes.KeyPath,
			md.cfg.EC2MasterNodes.Instances,
		)
		md.cfg.LogsMasterNodes = fpathToS3PathMasterNodes
		if err == nil {
			md.lg.Info("fetched master nodes logs")
		} else {
			md.lg.Warn("failed to fetch master nodes logs", zap.Error(err))
		}
	}

	if md.cfg.UploadTesterLogs && len(md.cfg.EC2WorkerNodes.Instances) > 0 && md.cfg.EC2WorkerNodesCreated {
		fpathToS3PathWorkerNodes, err := fetchLogs(
			md.lg,
			md.cfg.EC2MasterNodes.UserName,
			md.cfg.ClusterName,
			md.cfg.EC2MasterNodes.KeyPath,
			md.cfg.EC2MasterNodes.Instances,
		)
		md.cfg.LogsWorkerNodes = fpathToS3PathWorkerNodes
		if err == nil {
			md.lg.Info("fetched worker nodes logs")
		} else {
			md.lg.Warn("failed to fetch worker nodes logs", zap.Error(err))
		}

		err = md.uploadLogs()
		if err == nil {
			md.lg.Info("uploaded all nodes logs")
		} else {
			md.lg.Warn("failed to upload all nodes logs", zap.Error(err))
		}
	}

	ess := make([]string, 0)

	if md.cfg.LoadBalancerRegistered {
		instances := make([]*elb.Instance, 0, len(md.cfg.EC2MasterNodes.Instances)+len(md.cfg.EC2WorkerNodes.Instances))
		for _, iv := range md.cfg.EC2MasterNodes.Instances {
			instances = append(instances, &elb.Instance{
				InstanceId: aws.String(iv.InstanceID),
			})
		}
		for _, iv := range md.cfg.EC2WorkerNodes.Instances {
			instances = append(instances, &elb.Instance{
				InstanceId: aws.String(iv.InstanceID),
			})
		}
		if _, err := md.elbv1.DeregisterInstancesFromLoadBalancer(&elb.DeregisterInstancesFromLoadBalancerInput{
			LoadBalancerName: aws.String(md.cfg.LoadBalancerName),
			Instances:        instances,
		}); err != nil {
			md.lg.Warn("failed to de-register load balancer", zap.Error(err))
			ess = append(ess, err.Error())
		}
		md.cfg.LoadBalancerRegistered = false
		md.lg.Info("de-registered instances to load balancer", zap.String("name", md.cfg.LoadBalancerName), zap.Int("instances", len(instances)))
	}

	if md.cfg.LoadBalancerCreated {
		if _, err := md.elbv1.DeleteLoadBalancer(&elb.DeleteLoadBalancerInput{
			LoadBalancerName: aws.String(md.cfg.LoadBalancerName),
		}); err != nil {
			md.lg.Warn("failed to delete load balancer", zap.Error(err))
			ess = append(ess, err.Error())
		}
		md.cfg.LoadBalancerCreated = false
		md.lg.Info("deleted load balancer", zap.String("name", md.cfg.LoadBalancerName))
	}

	errc, cnt := make(chan error), 0
	var mu sync.Mutex
	if md.cfg.ETCDNodesCreated {
		cnt++
		// terminate etcd and worker nodes first in order to remove VPC dependency safely
		go func() {
			if err := md.etcdTester.Terminate(); err != nil {
				md.lg.Warn("failed to terminate etcd nodes", zap.Error(err))
				errc <- fmt.Errorf("failed to terminate etcd nodes (%v)", err)
				return
			}
			mu.Lock()
			md.cfg.ETCDNodesCreated = false
			mu.Unlock()
			errc <- nil
		}()
	}

	if md.cfg.EC2WorkerNodesCreated {
		cnt++
		go func() {
			if err := md.ec2WorkerNodesDeployer.Terminate(); err != nil {
				md.lg.Warn("failed to terminate EC2 worker nodes", zap.Error(err))
				errc <- fmt.Errorf("failed to terminate EC2 worker nodes (%v)", err)
				return
			}
			mu.Lock()
			md.cfg.EC2WorkerNodesCreated = false
			mu.Unlock()
			errc <- nil
		}()
	}
	for i := 0; i < cnt; i++ {
		if err := <-errc; err != nil {
			ess = append(ess, err.Error())
		}
	}
	if len(ess) == 0 {
		return nil
	}

	if md.cfg.EC2MasterNodesCreated {
		if err := md.ec2MasterNodesDeployer.Terminate(); err != nil {
			md.lg.Warn("failed to terminate EC2 master nodes", zap.Error(err))
			ess = append(ess, fmt.Sprintf("failed to terminate EC2 master nodes (%v)", err))
		} else {
			md.cfg.EC2MasterNodesCreated = false
		}
	}
	if len(ess) == 0 {
		return nil
	}
	return errors.New(strings.Join(ess, ", "))
}

func (md *embedded) uploadLogs() (err error) {
	ess := make([]string, 0)
	for k, v := range md.cfg.LogsMasterNodes {
		err = md.ec2MasterNodesDeployer.UploadToBucketForTests(k, v)
		if err != nil {
			md.lg.Warn("failed to upload kubernetes master node log", zap.String("file-path", k), zap.Error(err))
			ess = append(ess, err.Error())
		}
	}
	for k, v := range md.cfg.LogsWorkerNodes {
		err = md.ec2WorkerNodesDeployer.UploadToBucketForTests(k, v)
		if err != nil {
			md.lg.Warn("failed to upload kubernetes worker node log", zap.String("file-path", k), zap.Error(err))
			ess = append(ess, err.Error())
		}
	}
	if len(ess) == 0 {
		return nil
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
