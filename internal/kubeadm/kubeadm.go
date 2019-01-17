// Package kubeadm implements kubeadm test operations.
package kubeadm

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-k8s-tester/pkg/awsapi"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/kubeadmconfig"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
)

// Deployer defines kubeadm test operation.
type Deployer interface {
	Create() error
	Terminate() error
}

type embedded struct {
	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *kubeadmconfig.Config

	ec2MasterNodesDeployer ec2.Deployer
	ec2WorkerNodesDeployer ec2.Deployer

	ss    *session.Session
	elbv1 elbiface.ELBAPI     // for classic ELB
	elbv2 elbv2iface.ELBV2API // for ALB or NLB
}

// NewDeployer creates a new embedded kubeadm tester.
func NewDeployer(cfg *kubeadmconfig.Config) (Deployer, error) {
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

	md.cfg.ConfigPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.EC2MasterNodes.ConfigPathBucket)
	md.cfg.KubeConfigPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.KubeConfigPathBucket)
	md.cfg.LogOutputToUploadPathURL = genS3URL(md.cfg.AWSRegion, md.cfg.Tag, md.cfg.EC2MasterNodes.LogOutputToUploadPathBucket)

	defer func() {
		if err != nil {
			md.lg.Warn("failed to create Kubernetes, reverting", zap.Error(err))
			md.lg.Warn("failed to create Kubernetes, reverted", zap.Error(md.terminate()))
		}
	}()

	// shared master node VPC and subnets for: etcd nodes, worker nodes
	// do not run this in goroutine, since VPC for master nodes have to be created at first
	os.RemoveAll(md.cfg.EC2MasterNodes.KeyPath)
	if err = md.ec2MasterNodesDeployer.Create(); err != nil {
		return err
	}
	md.cfg.EC2MasterNodesCreated = true
	md.cfg.Sync()

	md.cfg.EC2WorkerNodes.VPCID = md.cfg.EC2MasterNodes.VPCID
	// prevent VPC double-delete
	md.cfg.EC2WorkerNodes.VPCCreated = false
	md.cfg.Sync()

	if err = md.ec2WorkerNodesDeployer.Create(); err != nil {
		return err
	}
	md.cfg.EC2WorkerNodesCreated = true
	md.cfg.Sync()

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
	md.cfg.LoadBalancerDNSName = *elbOut.DNSName
	md.cfg.LoadBalancerURL = md.cfg.LoadBalancerDNSName
	if !strings.HasPrefix(md.cfg.LoadBalancerURL, "https://") {
		md.cfg.LoadBalancerURL = "https://" + md.cfg.LoadBalancerURL
	}
	md.cfg.Sync()
	md.lg.Info("created load balancer", zap.String("name", md.cfg.LoadBalancerName), zap.String("dns-name", md.cfg.LoadBalancerDNSName))

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

	// TODO
	////////////////////////////////////////////////////////////////////////
	md.lg.Info("step 1-1. 'master node kubelet' configuration")
	md.lg.Info("step 1-2. successfully sent 'master node kubelet' environment file")
	md.lg.Info("step 1-3. successfully ran 'master node kubeadm init'")
	////////////////////////////////////////////////////////////////////////

	////////////////////////////////////////////////////////////////////////
	md.lg.Info("step 2-1. 'worker node kubelet' configuration")
	md.lg.Info("step 2-2. successfully sent 'worker node kubelet' environment file")
	md.lg.Info("step 3-3. successfully ran 'worker node kubeadm join'")
	////////////////////////////////////////////////////////////////////////

	/*
		for idx, target := range md.cfg.EC2.Instances {
			var kubeletEnvWorker string
			if kubeletEnvWorker, err = writeKubeletEnvFile(); err != nil {
				return err
			}
			defer os.RemoveAll(kubeletEnvWorker)
			md.lg.Info("successfully wrote 'kubelet' environment file", zap.String("index", idx), zap.String("private-dns", target.PrivateDNSName))
			if err = sendKubeletEnvFile(md.lg, *md.cfg.EC2, target, kubeletEnvWorker); err != nil {
				return err
			}
		}
		md.lg.Info("successfully sent 'kubelet' environment file")

		for idx, target := range md.cfg.EC2.Instances {
			if err = startKubeletService(md.lg, *md.cfg.EC2, target); err != nil {
				return err
			}
			md.lg.Info("successfully started 'kubelet' service", zap.String("index", idx), zap.String("private-dns", target.PrivateDNSName))
		}
		md.lg.Info("successfully started 'kubelet' service")
	*/

	// SCP to each EC2 instance
	// TODO: HA master
	md.lg.Info("running kubeadm init at master node")
	var masterEC2 ec2config.Instance
	for _, iv := range md.cfg.EC2.Instances {
		masterEC2 = iv
		break
	}
	md.cfg.Cluster.JoinTarget = fmt.Sprintf("%s:6443", masterEC2.PrivateIP)

	var masterSSH ssh.SSH
	masterSSH, err = ssh.New(ssh.Config{
		Logger:        md.lg,
		KeyPath:       md.cfg.EC2.KeyPath,
		PublicIP:      masterEC2.PublicIP,
		PublicDNSName: masterEC2.PublicDNSName,
		UserName:      md.cfg.EC2.UserName,
	})
	if err != nil {
		return err
	}
	if err = masterSSH.Connect(); err != nil {
		return err
	}
	defer masterSSH.Close()

	var script string
	script, err = md.cfg.Cluster.ScriptInit()
	if err != nil {
		return err
	}
	var localPath string
	localPath, err = fileutil.WriteTempFile([]byte(script))
	if err != nil {
		return err
	}
	defer os.RemoveAll(localPath)
	remotePath := fmt.Sprintf("/home/%s/kubeadm.init.sh", md.cfg.EC2.UserName)

	_, err = masterSSH.Send(
		localPath,
		remotePath,
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to send (%v)", err)
	}

	_, err = masterSSH.Run(
		fmt.Sprintf("chmod +x %s", remotePath),
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return err
	}

	_, err = masterSSH.Run(
		fmt.Sprintf("sudo bash %s &", remotePath),
		ssh.WithTimeout(15*time.Second),
	)
	md.lg.Info("started kubeadm init",
		zap.String("id", masterEC2.InstanceID),
		zap.Error(err),
	)

	kubeadmJoinCmd := ""
	retryStart := time.Now().UTC()
joinReady:
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var kubeadmInitOut []byte
		kubeadmInitOut, err = masterSSH.Run(
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
			md.lg.Info("waiting on kubeadm init")
			time.Sleep(15 * time.Second)
			continue
		}

		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if !strings.Contains(line, "--token") || !strings.Contains(line, "--discovery-token-ca-cert-hash") {
				continue
			}
			kubeadmJoinCmd = line
			break joinReady
		}
	}
	if kubeadmJoinCmd == "" {
		return errors.New("kubeadm join failed")
	}
	prevToken, prevHash := false, false
	for _, field := range strings.Fields(kubeadmJoinCmd) {
		if field == "--token" {
			prevToken = true
			continue
		}
		if prevToken {
			md.cfg.Cluster.JoinToken = field
			prevToken = false
			continue
		}
		if field == "--discovery-token-ca-cert-hash" {
			prevHash = true
			continue
		}
		if prevHash {
			md.cfg.Cluster.JoinDiscoveryTokenCACertHash = field
			prevHash = true
			continue
		}
	}
	var joinCmd string
	joinCmd, err = md.cfg.Cluster.CommandJoin()
	if err != nil {
		return err
	}
	md.lg.Info("kubeadm join command is ready", zap.String("command", joinCmd))

	md.lg.Info("checking kube-controller-manager")
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var dockerPsOutput []byte
		dockerPsOutput, err = masterSSH.Run(
			"sudo docker ps",
			ssh.WithRetry(15, 5*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		output := string(dockerPsOutput)
		fmt.Printf("\n\n%s\n\n", output)
		if strings.Contains(output, "kube-controller-manager") {
			break
		}
		md.lg.Info("waiting on kube-controller-manager")
		time.Sleep(15 * time.Second)
	}
	md.lg.Info("kube-controller-manager is ready")

	md.lg.Info("checking kube-controller-manager pod")
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var podsOutput []byte
		podsOutput, err = masterSSH.Run(
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
	md.lg.Info("kube-controller-manager pod is ready")

	var flannelOutputRole []byte
	flannelOutputRole, err = masterSSH.Run(
		"kubectl --kubeconfig=/home/ec2-user/.kube/config apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/k8s-manifests/kube-flannel-rbac.yml",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to apply flannel role (%v)", err)
	}
	fmt.Println("flannelOutputRole:", string(flannelOutputRole))
	var flannelOutput []byte
	flannelOutput, err = masterSSH.Run(
		"kubectl --kubeconfig=/home/ec2-user/.kube/config apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to apply flannel (%v)", err)
	}
	fmt.Println("flannelOutput:", string(flannelOutput))

	md.lg.Info("checking flannel pod")
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 10*time.Minute {
		var podsOutput []byte
		podsOutput, err = masterSSH.Run(
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
	md.lg.Info("flannel pod is ready")

	for id, iv := range md.cfg.EC2.Instances {
		if id == masterEC2.InstanceID {
			continue
		}
		md.lg.Info("node is joining master", zap.String("id", id))
		var nodeSSH ssh.SSH
		nodeSSH, err = ssh.New(ssh.Config{
			Logger:        md.lg,
			KeyPath:       md.cfg.EC2.KeyPath,
			PublicIP:      iv.PublicIP,
			PublicDNSName: iv.PublicDNSName,
			UserName:      md.cfg.EC2.UserName,
		})
		if err != nil {
			return err
		}
		if err = nodeSSH.Connect(); err != nil {
			return err
		}
		defer nodeSSH.Close()

		var joinOutput []byte
		joinOutput, err = nodeSSH.Run(
			joinCmd,
			ssh.WithRetry(15, 5*time.Second),
			ssh.WithTimeout(3*time.Minute),
		)
		if err != nil {
			return err
		}
		output := string(joinOutput)
		if !strings.Contains(output, "[discovery] Successfully established connection with API Server") || !strings.Contains(output, "This node has joined the cluster:") {
			return fmt.Errorf("failed to join cluster (%q)", output)
		}
		md.lg.Info("node has joined master", zap.String("id", id), zap.String("output", string(joinOutput)))
	}
	md.lg.Info("deployed kubeadm",
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	var nodesOutput []byte
	nodesOutput, err = masterSSH.Run(
		"kubectl --kubeconfig=/home/ec2-user/.kube/config get nodes",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return err
	}
	fmt.Println("nodesOutput:", string(nodesOutput))

	md.lg.Info("fetching KUBECONFIG", zap.String("KUBECONFIG", md.cfg.KubeConfigPath))
	var kubeconfigOutput []byte
	kubeconfigOutput, err = masterSSH.Run(
		"cat /home/ec2-user/.kube/config",
		ssh.WithRetry(15, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(md.cfg.KubeConfigPath, kubeconfigOutput, 0600)
	md.lg.Info("fetched KUBECONFIG", zap.String("KUBECONFIG", md.cfg.KubeConfigPath), zap.Error(err))

	if md.cfg.UploadTesterLogs {
		err = md.ec2MasterNodesDeployer.UploadToBucketForTests(md.cfg.KubeConfigPath, md.cfg.KubeConfigPathBucket)
		md.lg.Info("uploaded KUBECONFIG", zap.Error(err))

		var fpathToS3Path map[string]string
		fpathToS3Path, err = fetchLogs(
			md.lg,
			md.cfg.EC2.UserName,
			md.cfg.ClusterName,
			md.cfg.EC2.KeyPath,
			md.cfg.EC2.Instances,
		)
		md.cfg.Logs = fpathToS3Path
		err = md.uploadLogs()
		md.lg.Info("uploaded", zap.Error(err))
	}
	md.cfg.ValidateAndSetDefaults()

	return md.cfg.Sync()
}

func (md *embedded) Terminate() error {
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.terminate()
}

func (md *embedded) terminate() error {
	md.lg.Info("terminating kubeadm")
	if md.cfg.UploadTesterLogs && len(md.cfg.EC2.Instances) > 0 {
		err := md.ec2MasterNodesDeployer.UploadToBucketForTests(md.cfg.KubeConfigPath, md.cfg.KubeConfigPathBucket)
		md.lg.Info("uploaded KUBECONFIG", zap.Error(err))

		var fpathToS3Path map[string]string
		fpathToS3Path, err = fetchLogs(
			md.lg,
			md.cfg.EC2.UserName,
			md.cfg.ClusterName,
			md.cfg.EC2.KeyPath,
			md.cfg.EC2.Instances,
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
		md.lg.Info("uploaded kubeadm log", zap.Error(err))
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

	lg.Info("downloaded kubeadm log", zap.String("path", kubeletLogPath))
	fpathToS3Path = make(map[string]string)
	fpathToS3Path[kubeletLogPath] = fmt.Sprintf("%s/%s-kubelet.log", clusterName, id)
	return fpathToS3Path, nil
}

// genS3URL returns S3 URL path.
// e.g. https://s3-us-west-2.amazonaws.com/aws-k8s-tester-20180925/hello-world
func genS3URL(region, bucket, s3Path string) string {
	return fmt.Sprintf("https://s3-%s.amazonaws.com/%s/%s", region, bucket, s3Path)
}
