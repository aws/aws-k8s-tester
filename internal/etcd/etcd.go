// Package etcd implements etcd test operations.
package etcd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/etcdconfig"
	"github.com/aws/aws-k8s-tester/etcdtester"
	"github.com/aws/aws-k8s-tester/internal/ec2"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/zaputil"

	"github.com/dustin/go-humanize"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

type embedded struct {
	mu  sync.RWMutex
	lg  *zap.Logger
	cfg *etcdconfig.Config

	ec2Deployer        ec2.Deployer
	ec2BastionDeployer ec2.Deployer
}

// NewTester creates a new embedded etcd tester.
func NewTester(cfg *etcdconfig.Config) (etcdtester.Tester, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}

	lg, err := zaputil.New(cfg.LogDebug, cfg.LogOutputs)
	if err != nil {
		return nil, err
	}
	return &embedded{lg: lg, cfg: cfg}, cfg.Sync()
}

func (md *embedded) Create() (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	now := time.Now().UTC()

	// expect the following in Plugins
	// "update-amazon-linux-2"
	// "install-etcd-3.1.12"
	md.lg.Info(
		"deploying EC2",
		zap.Strings("plugins", md.cfg.EC2.Plugins),
	)
	md.ec2Deployer, err = ec2.NewDeployer(md.cfg.EC2)
	if err != nil {
		return err
	}
	if err = md.ec2Deployer.Create(); err != nil {
		return err
	}
	md.cfg.Sync()
	md.lg.Info(
		"deployed EC2",
		zap.Strings("plugins", md.cfg.EC2.Plugins),
		zap.String("vpc-id", md.cfg.EC2.VPCID),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	if md.cfg.LogDebug {
		fmt.Println(md.cfg.EC2.SSHCommands())
	}

	md.lg.Info(
		"deploying EC2 bastion",
		zap.Strings("plugins", md.cfg.EC2Bastion.Plugins),
	)
	md.cfg.EC2Bastion.VPCID = md.cfg.EC2.VPCID
	md.ec2BastionDeployer, err = ec2.NewDeployer(md.cfg.EC2Bastion)
	if err != nil {
		return err
	}
	if err = md.ec2BastionDeployer.Create(); err != nil {
		return err
	}
	md.lg.Info(
		"deployed EC2 bastion",
		zap.Strings("plugins", md.cfg.EC2Bastion.Plugins),
		zap.String("vpc-id", md.cfg.EC2Bastion.VPCID),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)
	if md.cfg.LogDebug {
		fmt.Println(md.cfg.EC2Bastion.SSHCommands())
	}

	tc := *md.cfg.Cluster
	for _, iv := range md.cfg.EC2.Instances {
		ev := tc
		ev.TopLevel = false
		ev.SSHPrivateKeyPath = md.cfg.EC2.KeyPath
		ev.PublicIP = iv.PublicIP
		ev.PublicDNSName = iv.PublicDNSName
		ev.Name = iv.InstanceID
		ev.DataDir = fmt.Sprintf("/home/%s/etcd.data", md.cfg.EC2.UserName)
		ev.ListenClientURLs = fmt.Sprintf("http://%s:2379", iv.PrivateIP)
		ev.AdvertiseClientURLs = fmt.Sprintf("http://%s:2379", iv.PrivateIP)
		ev.ListenPeerURLs = fmt.Sprintf("http://%s:2380", iv.PrivateIP)
		ev.AdvertisePeerURLs = fmt.Sprintf("http://%s:2380", iv.PrivateIP)
		ev.InitialCluster = ""
		ev.InitialClusterState = "new"
		md.cfg.ClusterState[iv.InstanceID] = ev
	}

	initialCluster := ""
	for k, v := range md.cfg.ClusterState {
		initialCluster += fmt.Sprintf(",%s=%s", k, v.AdvertisePeerURLs)
	}
	initialCluster = initialCluster[1:]

	for id, v := range md.cfg.ClusterState {
		v.InitialCluster = initialCluster
		md.cfg.ClusterState[id] = v
	}
	if err = md.cfg.ValidateAndSetDefaults(); err != nil {
		return err
	}

	// SCP to each EC2 instance
	// TODO: parallelize?
	md.lg.Info("deploying etcd",
		zap.String("initial-cluster", initialCluster),
	)
	for id, iv := range md.cfg.ClusterState {
		var svc string
		svc, err = iv.Service()
		if err != nil {
			return err
		}

		md.lg.Info("starting", zap.String("id", id), zap.String("public-dns-name", iv.PublicDNSName))
		var sh ssh.SSH
		sh, err = ssh.New(ssh.Config{
			Logger:        md.lg,
			KeyPath:       iv.SSHPrivateKeyPath,
			PublicIP:      iv.PublicIP,
			PublicDNSName: iv.PublicDNSName,
			UserName:      md.cfg.EC2.UserName,
		})
		if err != nil {
			return err
		}
		if err = sh.Connect(); err != nil {
			return err
		}

		var localPath string
		localPath, err = fileutil.WriteTempFile([]byte(svc))
		if err != nil {
			return err
		}
		defer os.RemoveAll(localPath)
		remotePath := fmt.Sprintf("/home/%s/etcd.sh", md.cfg.EC2.UserName)

		var out []byte
		out, err = sh.Send(
			localPath,
			remotePath,
			ssh.WithRetry(100, 5*time.Second),
			ssh.WithTimeout(30*time.Second),
		)
		if err != nil {
			return fmt.Errorf("failed to send %q (%v)", string(out), err)
		}

		_, err = sh.Run(
			fmt.Sprintf("chmod +x %s", remotePath),
			ssh.WithRetry(100, 5*time.Second),
			ssh.WithTimeout(30*time.Second),
		)
		if err != nil {
			return err
		}

		_, err = sh.Run(
			fmt.Sprintf("sudo bash %s", remotePath),
			ssh.WithTimeout(7*time.Second),
		)
		md.lg.Info("started", zap.String("id", id), zap.String("public-dns-name", iv.PublicDNSName), zap.Error(err))
	}
	md.lg.Info("deployed etcd",
		zap.String("initial-cluster", initialCluster),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	ready := 0
	for i := 0; i < 10; i++ {
		c := md.check()
		for id, v := range c.Members {
			md.lg.Info("counting status", zap.String("id", id), zap.String("status", v.Status))
			if v.OK {
				ready++
			}
		}
		if ready == md.cfg.ClusterSize {
			break
		}
		ready = 0
		time.Sleep(5 * time.Second)
	}

	if md.cfg.UploadTesterLogs {
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

	if ready != md.cfg.ClusterSize {
		return fmt.Errorf("cluster is not ready; expect %d ready, got %d", md.cfg.ClusterSize, ready)
	}
	return nil
}

func (md *embedded) Check() etcdtester.Cluster {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.check()
}

func (md *embedded) check() (c etcdtester.Cluster) {
	c.Members = make(map[string]etcdtester.Member, len(md.cfg.ClusterState))
	for k, v := range md.cfg.ClusterState {
		c.Members[k] = etcdtester.Member{
			ID:        k,
			ClientURL: v.AdvertiseClientURLs,
		}
	}

	var iv ec2config.Instance
	for _, v := range md.cfg.EC2Bastion.Instances {
		iv = v
		break
	}
	sh, err := ssh.New(ssh.Config{
		Logger:        md.lg,
		KeyPath:       md.cfg.EC2Bastion.KeyPath,
		UserName:      md.cfg.EC2Bastion.UserName,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
	})
	if err != nil {
		md.lg.Warn(
			"failed to create SSH",
			zap.Error(err),
		)
		for id := range md.cfg.ClusterState {
			vv, ok := c.Members[id]
			vv.ID = id
			if !ok {
				vv.Status = fmt.Sprintf("%s not found", id)
				vv.OK = false
			} else {
				vv.Status = fmt.Sprintf("failed to create SSH to bastion (%v)", err)
				vv.OK = false
			}
			c.Members[id] = vv
		}
		return c
	}

	if err = sh.Connect(); err != nil {
		md.lg.Warn(
			"failed to connect SSH",
			zap.Error(err),
		)
		for id := range md.cfg.ClusterState {
			vv, ok := c.Members[id]
			vv.ID = id
			if !ok {
				vv.Status = fmt.Sprintf("%s not found", id)
				vv.OK = false
			} else {
				vv.Status = fmt.Sprintf("failed to connect SSH to bastion (%v)", err)
				vv.OK = false
			}
			c.Members[id] = vv
		}
		return c
	}

	for id, v := range md.cfg.ClusterState {
		ep := v.AdvertiseClientURLs
		vv, ok := c.Members[id]
		vv.ID = id
		vv.ClientURL = ep
		if !ok {
			vv.Status = fmt.Sprintf("%s not found", id)
			vv.OK = false
			c.Members[id] = vv
			continue
		} else {
			vv.Status = fmt.Sprintf("failed to connect SSH to bastion (%v)", err)
			vv.OK = false
		}

		var out []byte
		out, err = sh.Run(
			fmt.Sprintf("curl -sL %s/health", ep),
			ssh.WithRetry(10, 3*time.Second),
			ssh.WithTimeout(5*time.Second),
		)
		if err != nil {
			vv.Status = fmt.Sprintf("status check for %q failed %v", ep, err)
			vv.OK = false
		} else {
			vv.Status = string(out)
			vv.OK = true
		}

		c.Members[id] = vv
	}
	return c
}

func (md *embedded) Cluster() (c etcdtester.Cluster) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.cluster()
}

func (md *embedded) cluster() (c etcdtester.Cluster) {
	c.Members = make(map[string]etcdtester.Member, len(md.cfg.ClusterState))
	for k, v := range md.cfg.ClusterState {
		c.Members[k] = etcdtester.Member{
			ID:        k,
			ClientURL: v.AdvertiseClientURLs,
		}
	}
	return c
}

func (md *embedded) Stop(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}

func (md *embedded) Restart(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}

func (md *embedded) Terminate() error {
	md.mu.Lock()
	defer md.mu.Unlock()

	if md.cfg.UploadTesterLogs && len(md.cfg.ClusterState) > 0 {
		fpathToS3Path, err := fetchLogs(
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

	errc := make(chan error)
	go func() {
		errc <- md.ec2Deployer.Terminate()

	}()
	go func() {
		errc <- md.ec2BastionDeployer.Terminate()
	}()
	errEC2 := <-errc
	errEC2Bastion := <-errc

	ev := ""
	if errEC2 != nil {
		ev += fmt.Sprintf(",failed to terminate etcd EC2 instances (%v)", errEC2)
	}
	if errEC2Bastion != nil {
		ev += fmt.Sprintf(",failed to terminate etcd bastion EC2 instances (%v)", errEC2Bastion)
	}
	if ev != "" {
		return errors.New(ev[1:])
	}
	return nil
}

func (md *embedded) MemberAdd(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}

func (md *embedded) MemberRemove(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	return nil
}

func (md *embedded) uploadLogs() (err error) {
	ess := []string{}
	for k, v := range md.cfg.Logs {
		err = md.ec2Deployer.UploadToBucketForTests(k, v)
		md.lg.Info("uploaded etcd log", zap.Error(err))
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

// TODO: get more system level logs, disk stats?
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

	var out []byte
	out, err = sh.Run(
		"sudo journalctl --no-pager -u etcd.service",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(3*time.Minute),
	)
	if err != nil {
		return nil, err
	}
	var etcdLogPath string
	etcdLogPath, err = fileutil.WriteTempFile(out)
	if err != nil {
		return nil, err
	}

	lg.Info("downloaded etcd log", zap.String("path", etcdLogPath))
	fpathToS3Path = make(map[string]string)
	fpathToS3Path[etcdLogPath] = fmt.Sprintf("%s/%s-etcd.server.log", clusterName, id)
	return fpathToS3Path, nil
}

// TODO
func (md *embedded) checkETCDHealth() {
	_ = clientv3.Config{}

	// hh = make(map[string]etcdtester.Health, len(md.cfg.ClusterState))
	// for k, v := range md.cfg.ClusterState {
	// 	ep := v.AdvertiseClientURLs
	// 	health := etcdtester.Health{
	// 		Status: "",
	// 		Error:  nil,
	// 	}
	// 	cli, err := clientv3.New(clientv3.Config{
	// 		Endpoints: []string{ep},
	// 	})
	// 	if err != nil {
	// 		health.Status = fmt.Sprintf("status check for %q failed %v", ep, err)
	// 		vv.OK = false
	// 	} else {
	// 		defer cli.Close()
	// 		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	// 		sresp, serr := cli.Status(ctx, ep)
	// 		cancel()
	// 		if serr != nil {
	// 			health.Status = fmt.Sprintf("status check for %q failed %v", ep, serr)
	// 			health.Error = serr
	// 		} else {
	// 			health.Status = fmt.Sprintf("status check for %q: %+v", ep, sresp)
	// 			health.Error = nil
	// 		}
	// 	}
	// 	hh[k] = health
	// }
	// return hh
}
