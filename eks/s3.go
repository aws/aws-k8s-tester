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
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-writes-raw.json"),
				ts.cfg.AddOnCSRsLocal.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-summary-writes.json"),
				ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-summary-writes.txt"),
				ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-summary-writes-compare.json"),
				ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-local-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnCSRsLocal.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnCSRsRemote() {
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-writes-raw.json"),
				ts.cfg.AddOnCSRsRemote.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-summary-writes.json"),
				ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-summary-writes-compare.json"),
				ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "csrs-remote-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnCSRsRemote.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConfigmapsLocal() {
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-writes-raw.json"),
				ts.cfg.AddOnConfigmapsLocal.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-summary-writes.json"),
				ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-summary-writes.txt"),
				ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-summary-writes-compare.json"),
				ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-local-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnConfigmapsLocal.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnConfigmapsRemote() {
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-writes-raw.json"),
				ts.cfg.AddOnConfigmapsRemote.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-summary-writes.json"),
				ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-summary-writes-compare.json"),
				ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "configmaps-remote-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnConfigmapsRemote.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnSecretsLocal() {
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-writes-raw.json"),
				ts.cfg.AddOnSecretsLocal.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-writes.json"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-writes.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-writes-compare.json"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsRawReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-raw-reads.json"),
				ts.cfg.AddOnSecretsLocal.RequestsRawReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-reads.json"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-reads.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-reads-compare.json"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-local-requests-summary-reads-compare.txt"),
				ts.cfg.AddOnSecretsLocal.RequestsSummaryReadsCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnSecretsRemote() {
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-writes-raw.json"),
				ts.cfg.AddOnSecretsRemote.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-writes.json"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-writes-compare.json"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsRawReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-raw-reads.json"),
				ts.cfg.AddOnSecretsRemote.RequestsRawReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-reads.json"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-reads.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-reads-compare.json"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "secrets-remote-requests-summary-reads-compare.txt"),
				ts.cfg.AddOnSecretsRemote.RequestsSummaryReadsCompareTablePath,
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
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-writes-raw.json"),
				ts.cfg.AddOnStresserLocal.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-writes.json"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-writes.txt"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-writes-compare.json"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsRawReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-raw-reads.json"),
				ts.cfg.AddOnStresserLocal.RequestsRawReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-reads.json"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-reads.txt"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryReadsTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryReadsCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-reads-compare.json"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryReadsCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserLocal.RequestsSummaryReadsCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-local-requests-summary-reads-compare.txt"),
				ts.cfg.AddOnStresserLocal.RequestsSummaryReadsCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	if ts.cfg.IsEnabledAddOnStresserRemote() {
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsRawWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-writes-raw.json"),
				ts.cfg.AddOnStresserRemote.RequestsRawWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-writes.json"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryWritesJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-writes.txt"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryWritesTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryWritesCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-writes-compare.json"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryWritesCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryWritesCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-writes-compare.txt"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryWritesCompareTablePath,
			); err != nil {
				return err
			}
		}

		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsRawReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-raw-reads.json"),
				ts.cfg.AddOnStresserRemote.RequestsRawReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-reads.json"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryReadsJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-reads.txt"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryReadsTablePath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryReadsCompareJSONPath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-reads-compare.json"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryReadsCompareJSONPath,
			); err != nil {
				return err
			}
		}
		if fileutil.Exist(ts.cfg.AddOnStresserRemote.RequestsSummaryReadsCompareTablePath) {
			if err = aws_s3.Upload(
				ts.lg,
				ts.s3API,
				ts.cfg.S3BucketName,
				path.Join(ts.cfg.Name, "stresser-remote-requests-summary-reads-compare.txt"),
				ts.cfg.AddOnStresserRemote.RequestsSummaryReadsCompareTablePath,
			); err != nil {
				return err
			}
		}
	}

	return err
}
