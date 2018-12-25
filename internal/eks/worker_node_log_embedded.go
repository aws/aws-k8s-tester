package eks

import (
	"errors"

	"go.uber.org/zap"
)

func (md *embedded) GetWorkerNodeLogs() (err error) {
	if !md.cfg.EnableWorkerNodeSSH {
		return errors.New("node SSH is not enabled")
	}

	var fpathToS3Path map[string]string
	fpathToS3Path, err = fetchWorkerNodeLogs(
		md.lg,
		"ec2-user", // for Amazon Linux 2
		md.cfg.ClusterName,
		md.cfg.WorkerNodePrivateKeyPath,
		md.cfg.ClusterState.WorkerNodes,
	)

	md.ec2InstancesLogMu.Lock()
	md.cfg.ClusterState.WorkerNodeLogs = fpathToS3Path
	md.ec2InstancesLogMu.Unlock()

	md.cfg.Sync()
	md.lg.Info("updated worker node logs", zap.String("synced-config-path", md.cfg.ConfigPath))

	return nil
}
