// Package etcd implements etcd test operations.
package etcd

import (
	"encoding/json"
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
	"go.etcd.io/etcd/etcdserver/etcdserverpb"
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
		defer sh.Close()

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
			ssh.WithTimeout(15*time.Second),
		)
		if err != nil {
			return fmt.Errorf("failed to send %q (%v)", string(out), err)
		}

		_, err = sh.Run(
			fmt.Sprintf("chmod +x %s", remotePath),
			ssh.WithRetry(100, 5*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		if err != nil {
			return err
		}

		_, err = sh.Run(
			fmt.Sprintf("sudo bash %s", remotePath),
			ssh.WithTimeout(15*time.Second),
		)
		md.lg.Info("started", zap.String("id", id), zap.String("public-dns-name", iv.PublicDNSName), zap.Error(err))
	}
	md.lg.Info("deployed etcd",
		zap.String("initial-cluster", initialCluster),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	ready := 0
	for i := 0; i < 10; i++ {
		c := md.checkHealth()
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
	md.memberList()
	md.cfg.Sync()

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

func (md *embedded) Cluster() (c etcdtester.Cluster) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.cluster()
}

func (md *embedded) cluster() (c etcdtester.Cluster) {
	c.Members = make(map[string]etcdtester.Member, len(md.cfg.ClusterState))
	for id, v := range md.cfg.ClusterState {
		c.Members[id] = etcdtester.Member{
			ID:        id,
			ClientURL: v.AdvertiseClientURLs,
		}
	}
	return c
}

func (md *embedded) CheckHealth() etcdtester.Cluster {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.checkHealth()
}

func (md *embedded) checkHealth() (c etcdtester.Cluster) {
	c.Members = make(map[string]etcdtester.Member, len(md.cfg.ClusterState))
	for id, v := range md.cfg.ClusterState {
		c.Members[id] = etcdtester.Member{
			ID:        id,
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

	md.lg.Info("connecting to EC2 bastion to check '/health'")
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
	defer sh.Close()

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
			ssh.WithTimeout(15*time.Second),
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

func (md *embedded) CheckStatus() (c etcdtester.Cluster) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.checkStatus()
}

func (md *embedded) checkStatus() (c etcdtester.Cluster) {
	md.cfg.Sync()

	c.Members = make(map[string]etcdtester.Member, len(md.cfg.ClusterState))
	for id, v := range md.cfg.ClusterState {
		c.Members[id] = etcdtester.Member{
			ID:        id,
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

	md.lg.Info("connecting to EC2 bastion to run 'member status'")
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
	defer sh.Close()

	remotePath := fmt.Sprintf("/home/%s/aws-k8s-tester.etcd.yaml", md.cfg.EC2Bastion.UserName)
	_, err = sh.Send(
		md.cfg.ConfigPath,
		remotePath,
		ssh.WithRetry(10, 3*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		for id, m := range c.Members {
			m.Status = fmt.Sprintf("status check failed to send configuration file (%v)", err)
			m.OK = false
			c.Members[id] = m
		}
	} else {
		var out []byte
		out, err = sh.Run(
			fmt.Sprintf("aws-k8s-tester etcd test status --path=%s", remotePath),
			ssh.WithRetry(10, 3*time.Second),
			ssh.WithTimeout(15*time.Second),
		)
		if err != nil {
			for id, m := range c.Members {
				m.Status = fmt.Sprintf("status check failed to run 'test status' commands (%v)", err)
				m.OK = false
				c.Members[id] = m
			}
		} else {
			c2 := etcdtester.Cluster{}
			err = json.Unmarshal(out, &c2)
			if err != nil {
				for id, m := range c.Members {
					m.Status = fmt.Sprintf("status check failed to decode 'test status' output (%v)", err)
					m.OK = false
					c.Members[id] = m
				}
			} else {
				c = c2
			}
		}
	}
	return c
}

func (md *embedded) MemberList() (*etcdserverpb.MemberListResponse, error) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.memberList()
}

func (md *embedded) memberList() (*etcdserverpb.MemberListResponse, error) {
	md.cfg.Sync()

	md.lg.Info("getting member list")
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
		return nil, err
	}
	md.lg.Info("connecting to EC2 bastion to run 'member list'")
	if err = sh.Connect(); err != nil {
		return nil, err
	}
	defer sh.Close()

	remotePath := fmt.Sprintf("/home/%s/aws-k8s-tester.etcd.yaml", md.cfg.EC2Bastion.UserName)
	_, err = sh.Send(
		md.cfg.ConfigPath,
		remotePath,
		ssh.WithRetry(10, 3*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return nil, err
	}

	var out []byte
	out, err = sh.Run(
		fmt.Sprintf("aws-k8s-tester etcd test member list --path=%s", remotePath),
		ssh.WithRetry(10, 3*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		return nil, err
	}

	// presp.Unmarshal(out) if marshaled via proto
	// in "aws-k8s-tester etcd test member list"
	presp := &etcdserverpb.MemberListResponse{}
	if err = json.Unmarshal(out, presp); err != nil {
		return nil, err
	}
	for _, mem := range presp.Members {
		nameID, memberID := "", ""
		for id, v := range md.cfg.ClusterState {
			if v.AdvertiseClientURLs == mem.ClientURLs[0] {
				nameID, memberID = id, fmt.Sprintf("%x", mem.ID)
			}
		}
		if nameID == "" || memberID == "" {
			return nil, fmt.Errorf("no cluster state ETCD found for member %+v", mem)
		}
		v, ok := md.cfg.ClusterState[nameID]
		if !ok {
			return nil, fmt.Errorf("no cluster state ETCD found for name ID %q", nameID)
		}
		v.MemberID = memberID
		md.cfg.ClusterState[nameID] = v
	}
	return presp, md.cfg.Sync()
}

func (md *embedded) Stop(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	md.lg.Info("stopping etcd", zap.String("id", id))

	_, ok := md.cfg.ClusterState[id]
	if !ok {
		return fmt.Errorf("%q does not exist, can't stop", id)
	}
	var iv ec2config.Instance
	iv, ok = md.cfg.EC2.Instances[id]
	if !ok {
		return fmt.Errorf("%q does not exist, can't stop", id)
	}

	sh, err := ssh.New(ssh.Config{
		Logger:        md.lg,
		KeyPath:       md.cfg.EC2.KeyPath,
		UserName:      md.cfg.EC2.UserName,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
	})
	if err != nil {
		return err
	}
	md.lg.Info("connecting to EC2 instance to stop")
	if err = sh.Connect(); err != nil {
		return err
	}
	defer sh.Close()

	_, err = sh.Run(
		"sudo systemctl stop etcd.service",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		md.lg.Info("failed to stop etcd", zap.String("id", id))
	}

	c := md.checkHealth()
	for id, v := range c.Members {
		md.lg.Info("checked health status after stop", zap.String("id", id), zap.String("status", v.Status))
	}
	return err
}

func (md *embedded) Restart(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	md.lg.Info("restarting etcd", zap.String("id", id))

	_, ok := md.cfg.ClusterState[id]
	if !ok {
		return fmt.Errorf("%q does not exist, can't restart", id)
	}
	var iv ec2config.Instance
	iv, ok = md.cfg.EC2.Instances[id]
	if !ok {
		return fmt.Errorf("%q does not exist, can't restart", id)
	}

	sh, err := ssh.New(ssh.Config{
		Logger:        md.lg,
		KeyPath:       md.cfg.EC2.KeyPath,
		UserName:      md.cfg.EC2.UserName,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
	})
	if err != nil {
		return err
	}
	md.lg.Info("connecting to EC2 instance to restart")
	if err = sh.Connect(); err != nil {
		return err
	}
	defer sh.Close()

	_, err = sh.Run(
		"sudo systemctl enable etcd.service && sudo systemctl start etcd.service &",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(3*time.Minute),
	)
	if err != nil {
		md.lg.Info("failed to restart etcd", zap.String("id", id), zap.Error(err))
	}
	c := md.checkHealth()
	for id, v := range c.Members {
		md.lg.Info("checked health status after restart", zap.String("id", id), zap.String("status", v.Status))
	}
	return err
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

func (md *embedded) MemberRemove(id string) (err error) {
	md.mu.Lock()
	defer md.mu.Unlock()

	md.lg.Info("removing etcd", zap.String("id", id))
	if _, err = md.memberList(); err != nil {
		return err
	}

	e, ok := md.cfg.ClusterState[id]
	if !ok {
		return fmt.Errorf("%q does not exist, can't remove", id)
	}
	memberID := e.MemberID

	_, ok = md.cfg.EC2.Instances[id]
	if !ok {
		return fmt.Errorf("%q does not exist, can't remove", id)
	}

	var iv ec2config.Instance
	for _, v := range md.cfg.EC2Bastion.Instances {
		iv = v
		break
	}
	var sh ssh.SSH
	sh, err = ssh.New(ssh.Config{
		Logger:        md.lg,
		KeyPath:       md.cfg.EC2Bastion.KeyPath,
		UserName:      md.cfg.EC2Bastion.UserName,
		PublicIP:      iv.PublicIP,
		PublicDNSName: iv.PublicDNSName,
	})
	if err != nil {
		return err
	}
	md.lg.Info("connecting to EC2 bastion to run 'member remove' command")
	if err = sh.Connect(); err != nil {
		return err
	}
	defer sh.Close()

	eps := []string{}
	for id2, v := range md.cfg.ClusterState {
		if id == id2 {
			continue
		}
		eps = append(eps, v.AdvertiseClientURLs)
	}

	/*
		ETCDCTL_API=3 etcdctl --endpoints=http://192.168.182.84:2379,http://192.168.65.236:2379 member remove 6880345acaba6c00
		Member 6880345acaba6c00 removed from cluster 3f9b5afcc7c33e1c
	*/
	var out []byte
	out, err = sh.Run(
		fmt.Sprintf("ETCDCTL_API=3 etcdctl --endpoints=%s member remove %s", strings.Join(eps, ","), memberID),
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
	)
	if err != nil {
		md.lg.Warn("failed to member remove", zap.String("id", id), zap.Error(err))
	} else if strings.Contains(string(out), "removed from cluster") {
		md.lg.Info("removed member", zap.String("id", id), zap.String("output", string(out)))
		delete(md.cfg.ClusterState, id)
		md.cfg.ClusterSize--
	} else {
		md.lg.Warn("failed to member remove", zap.String("id", id), zap.String("output", string(out)))
		return fmt.Errorf("failed to remove member %q (member ID %q, output %s)", id, memberID, string(out))
	}

	defer func() {
		fmt.Println("after delete, len(md.cfg.ClusterState):", len(md.cfg.ClusterState))
		fmt.Println("after delete, md.cfg.ClusterSize:", md.cfg.ClusterSize)
		fmt.Println("after delete, len(md.cfg.EC2.Instances):", len(md.cfg.EC2.Instances))
		fmt.Println("after delete, md.cfg.EC2.ClusterSize:", md.cfg.EC2.ClusterSize)
	}()

	md.cfg.Sync()
	return md.ec2Deployer.Delete(id)
}

func (md *embedded) MemberAdd(id string) error {
	md.mu.Lock()
	defer md.mu.Unlock()

	_, ok := md.cfg.ClusterState[id]
	if ok {
		return fmt.Errorf("%q already exist, can't add", id)
	}
	var iv ec2config.Instance
	iv, ok = md.cfg.EC2.Instances[id]
	if ok {
		return fmt.Errorf("%q already exist, can't add", id)
	}
	_ = iv

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
	defer sh.Close()

	var out []byte
	out, err = sh.Run(
		"sudo journalctl --no-pager -u etcd.service",
		ssh.WithRetry(100, 5*time.Second),
		ssh.WithTimeout(15*time.Second),
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
