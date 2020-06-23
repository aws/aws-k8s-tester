package eks

import (
	"errors"
	"os"
	"path"
	"path/filepath"

	aws_s3 "github.com/aws/aws-k8s-tester/pkg/aws/s3"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"go.uber.org/zap"
)

func (ts *Tester) createS3() (err error) {
	if ts.cfg.S3BucketCreate {
		if ts.cfg.S3BucketName == "" {
			return errors.New("empty S3 bucket name")
		}
		if err = aws_s3.CreateBucket(ts.lg, ts.s3API, ts.cfg.S3BucketName, ts.cfg.Region, ts.cfg.Name, ts.cfg.S3BucketLifecycleExpirationDays); err != nil {
			return err
		}
	} else {
		ts.lg.Info("skipping S3 bucket creation")
	}
	if ts.cfg.S3BucketName == "" {
		ts.lg.Info("skipping s3 bucket creation")
		return nil
	}
	return ts.cfg.Sync()
}

func (ts *Tester) deleteS3() error {
	if !ts.cfg.S3BucketCreate {
		ts.lg.Info("skipping S3 bucket deletion", zap.String("s3-bucket-name", ts.cfg.S3BucketName))
		return nil
	}
	if ts.cfg.S3BucketCreateKeep {
		ts.lg.Info("skipping S3 bucket deletion", zap.String("s3-bucket-name", ts.cfg.S3BucketName), zap.Bool("s3-bucket-create-keep", ts.cfg.S3BucketCreateKeep))
		return nil
	}
	if err := aws_s3.EmptyBucket(ts.lg, ts.s3API, ts.cfg.S3BucketName); err != nil {
		return err
	}
	return aws_s3.DeleteBucket(ts.lg, ts.s3API, ts.cfg.S3BucketName)
}

func (ts *Tester) uploadToS3() (err error) {
	if ts.cfg.S3BucketName == "" {
		ts.lg.Info("skipping s3 uploads; s3 bucket name is empty")
		return nil
	}

	if fileutil.Exist(ts.cfg.ConfigPath) {
		if err = aws_s3.Upload(
			ts.lg,
			ts.s3API,
			ts.cfg.S3BucketName,
			path.Join(ts.cfg.Name, "aws-k8s-tester-eks.config.yaml"),
			ts.cfg.ConfigPath,
		); err != nil {
			return err
		}
	}

	logFilePath := ""
	for _, fpath := range ts.cfg.LogOutputs {
		if filepath.Ext(fpath) == ".log" {
			logFilePath = fpath
			break
		}
	}
	if fileutil.Exist(logFilePath) {
		if err = aws_s3.Upload(
			ts.lg,
			ts.s3API,
			ts.cfg.S3BucketName,
			path.Join(ts.cfg.Name, "aws-k8s-tester-eks.log"),
			logFilePath,
		); err != nil {
			return err
		}
	}

	if fileutil.Exist(ts.cfg.KubeConfigPath) {
		if err = aws_s3.Upload(
			ts.lg,
			ts.s3API,
			ts.cfg.S3BucketName,
			path.Join(ts.cfg.Name, "kubeconfig.yaml"),
			ts.cfg.KubeConfigPath,
		); err != nil {
			return err
		}
	}

	if fileutil.Exist(ts.cfg.Status.ClusterMetricsRawOutputDir) {
		err = filepath.Walk(ts.cfg.Status.ClusterMetricsRawOutputDir, func(path string, info os.FileInfo, werr error) error {
			if werr != nil {
				return werr
			}
			if info.IsDir() {
				return nil
			}
			if uerr := aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				filepath.Join(ts.cfg.Name, "metrics", filepath.Base(path)),
				path,
			); uerr != nil {
				return uerr
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	if ts.cfg.IsEnabledAddOnNodeGroups() {
		if fileutil.Exist(ts.cfg.AddOnNodeGroups.RoleCFNStackYAMLFilePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", "add-on-node-groups.role.cfn.yaml"),
				ts.cfg.AddOnNodeGroups.RoleCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLFilePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", "add-on-node-groups.sg.cfn.yaml"),
				ts.cfg.AddOnNodeGroups.NodeGroupSecurityGroupCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnNodeGroups.LogsTarGzPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "add-on-node-groups-logs-dir.tar.gz"),
				ts.cfg.AddOnNodeGroups.LogsTarGzPath,
			); err != nil {
				return err
			}
		}
		for asgName, cur := range ts.cfg.AddOnNodeGroups.ASGs {
			if fileutil.Exist(cur.ASGCFNStackYAMLFilePath) {
				if err = aws_s3.Upload(
					ts.lg,
					ts.s3API,
					ts.cfg.S3BucketName,
					path.Join(ts.cfg.Name, "cfn", "add-on-node-groups.asg.cfn."+asgName+".yaml"),
					cur.ASGCFNStackYAMLFilePath,
				); err != nil {
					return err
				}
			}
			if fileutil.Exist(cur.SSMDocumentCFNStackYAMLFilePath) {
				if err = aws_s3.Upload(
					ts.lg,
					ts.s3API,
					ts.cfg.S3BucketName,
					path.Join(ts.cfg.Name, "cfn", "add-on-node-groups.ssm.cfn."+asgName+".yaml"),
					cur.SSMDocumentCFNStackYAMLFilePath,
				); err != nil {
					return err
				}
			}
		}
	}

	if ts.cfg.IsEnabledAddOnManagedNodeGroups() {
		if fileutil.Exist(ts.cfg.AddOnManagedNodeGroups.RoleCFNStackYAMLFilePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", "add-on-managed-node-groups.role.cfn.yaml"),
				ts.cfg.AddOnManagedNodeGroups.RoleCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnManagedNodeGroups.LogsTarGzPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "add-on-managed-node-groups-logs-dir.tar.gz"),
				ts.cfg.AddOnManagedNodeGroups.LogsTarGzPath,
			); err != nil {
				return err
			}
		}
		for mngName, cur := range ts.cfg.AddOnManagedNodeGroups.MNGs {
			if fileutil.Exist(cur.MNGCFNStackYAMLFilePath) {
				if err = aws_s3.Upload(
					ts.lg,
					ts.s3API,
					ts.cfg.S3BucketName,
					path.Join(ts.cfg.Name, "cfn", "add-on-managed-node-groups.mng.cfn."+mngName+".yaml"),
					cur.MNGCFNStackYAMLFilePath,
				); err != nil {
					return err
				}
			}
			if fileutil.Exist(cur.RemoteAccessSecurityCFNStackYAMLFilePath) {
				if err = aws_s3.Upload(
					ts.lg,
					ts.s3API,
					ts.cfg.S3BucketName,
					path.Join(ts.cfg.Name, "cfn", "add-on-managed-node-groups.sg.cfn."+mngName+".yaml"),
					cur.RemoteAccessSecurityCFNStackYAMLFilePath,
				); err != nil {
					return err
				}
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConformance() {
		if fileutil.Exist(ts.cfg.AddOnConformance.SonobuoyResultTarGzPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "sonobuoy-result.tar.gz"),
				ts.cfg.AddOnConformance.SonobuoyResultTarGzPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConformance.SonobuoyResultE2eLogPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "sonobuoy-result.e2e.log"),
				ts.cfg.AddOnConformance.SonobuoyResultE2eLogPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConformance.SonobuoyResultJunitXMLPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "sonobuoy-result.junit.xml"),
				ts.cfg.AddOnConformance.SonobuoyResultJunitXMLPath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnAppMesh() {
		if fileutil.Exist(ts.cfg.AddOnAppMesh.PolicyCFNStackYAMLFilePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", "add-on-app-mesh.policy.cfn.yaml"),
				ts.cfg.AddOnAppMesh.PolicyCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnCSRsLocal() {
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-writes.json"),
				ts.cfg.AddOnCSRsLocal.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-writes-summary.json"),
				ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-writes-summary.txt"),
				ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-writes-summary-compare.json"),
				ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnCSRsLocal.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnCSRsRemote() {
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-writes.json"),
				ts.cfg.AddOnCSRsRemote.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-writes-summary.json"),
				ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-writes-summary.txt"),
				ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-writes-summary-compare.json"),
				ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnCSRsRemote.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConfigmapsLocal() {
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-writes.json"),
				ts.cfg.AddOnConfigmapsLocal.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-writes-summary.json"),
				ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-writes-summary.txt"),
				ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-writes-summary-compare.json"),
				ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnConfigmapsLocal.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConfigmapsRemote() {
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-writes.json"),
				ts.cfg.AddOnConfigmapsRemote.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-writes-summary.json"),
				ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-writes-summary.txt"),
				ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-writes-summary-compare.json"),
				ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnConfigmapsRemote.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnSecretsLocal() {
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-writes.json"),
				ts.cfg.AddOnSecretsLocal.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-writes-summary.json"),
				ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-writes-summary.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-writes-summary-compare.json"),
				ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-reads.json"),
				ts.cfg.AddOnSecretsLocal.RequestsReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-reads-summary.json"),
				ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-reads-summary.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-reads-summary-compare.json"),
				ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-reads-summary-compare.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsReadsSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnSecretsRemote() {
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-writes.json"),
				ts.cfg.AddOnSecretsRemote.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-writes-summary.json"),
				ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-writes-summary.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-writes-summary-compare.json"),
				ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-reads.json"),
				ts.cfg.AddOnSecretsRemote.RequestsReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-reads-summary.json"),
				ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-reads-summary.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-reads-summary-compare.json"),
				ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-reads-summary-compare.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsReadsSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnFargate() {
		if fileutil.Exist(ts.cfg.AddOnFargate.RoleCFNStackYAMLFilePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", "add-on-fargate.role.cfn.yaml"),
				ts.cfg.AddOnFargate.RoleCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnIRSA() {
		if fileutil.Exist(ts.cfg.AddOnIRSA.RoleCFNStackYAMLFilePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", "add-on-irsa.role.cfn.yaml"),
				ts.cfg.AddOnIRSA.RoleCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnIRSAFargate() {
		if fileutil.Exist(ts.cfg.AddOnIRSAFargate.RoleCFNStackYAMLFilePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cfn", "add-on-irsa-fargate.role.cfn.yaml"),
				ts.cfg.AddOnIRSAFargate.RoleCFNStackYAMLFilePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnClusterLoaderLocal() {
		if fileutil.Exist(ts.cfg.AddOnClusterLoaderLocal.ReportTarGzPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cluster-loader-local.tar.gz"),
				ts.cfg.AddOnClusterLoaderLocal.ReportTarGzPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnClusterLoaderLocal.LogPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cluster-loader-local.log"),
				ts.cfg.AddOnClusterLoaderLocal.LogPath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnClusterLoaderRemote() {
		if fileutil.Exist(ts.cfg.AddOnClusterLoaderRemote.ReportTarGzPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cluster-loader-remote.tar.gz"),
				ts.cfg.AddOnClusterLoaderRemote.ReportTarGzPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnClusterLoaderRemote.LogPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "cluster-loader-remote.log"),
				ts.cfg.AddOnClusterLoaderRemote.LogPath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnStresserLocal() {
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-writes.json"),
				ts.cfg.AddOnStresserLocal.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-writes-summary.json"),
				ts.cfg.AddOnStresserLocal.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-writes-summary.txt"),
				ts.cfg.AddOnStresserLocal.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-writes-summary-compare.json"),
				ts.cfg.AddOnStresserLocal.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnStresserLocal.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-reads.json"),
				ts.cfg.AddOnStresserLocal.RequestsReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsReadsSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-reads-summary.json"),
				ts.cfg.AddOnStresserLocal.RequestsReadsSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsReadsSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-reads-summary.txt"),
				ts.cfg.AddOnStresserLocal.RequestsReadsSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsReadsSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-reads-summary-compare.json"),
				ts.cfg.AddOnStresserLocal.RequestsReadsSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsReadsSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-reads-summary-compare.txt"),
				ts.cfg.AddOnStresserLocal.RequestsReadsSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnStresserRemote() {
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-writes.json"),
				ts.cfg.AddOnStresserRemote.RequestsWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsWritesSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-writes-summary.json"),
				ts.cfg.AddOnStresserRemote.RequestsWritesSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsWritesSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-writes-summary.txt"),
				ts.cfg.AddOnStresserRemote.RequestsWritesSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsWritesSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-writes-summary-compare.json"),
				ts.cfg.AddOnStresserRemote.RequestsWritesSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsWritesSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-writes-summary-compare.txt"),
				ts.cfg.AddOnStresserRemote.RequestsWritesSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-reads.json"),
				ts.cfg.AddOnStresserRemote.RequestsReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsReadsSummaryJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-reads-summary.json"),
				ts.cfg.AddOnStresserRemote.RequestsReadsSummaryJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsReadsSummaryTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-reads-summary.txt"),
				ts.cfg.AddOnStresserRemote.RequestsReadsSummaryTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsReadsSummaryCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-reads-summary-compare.json"),
				ts.cfg.AddOnStresserRemote.RequestsReadsSummaryCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsReadsSummaryCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-reads-summary-compare.txt"),
				ts.cfg.AddOnStresserRemote.RequestsReadsSummaryCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	return err
}
