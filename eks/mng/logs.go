package mng

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/aws/aws-k8s-tester/ssh"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dustin/go-humanize"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var defaultLogs = map[string]string{
	// kernel logs
	"sudo journalctl --no-pager --output=short-precise -k": "kernel.out.log",

	// full journal logs (e.g. disk mounts)
	"sudo journalctl --no-pager --output=short-precise": "journal.out.log",

	// other systemd services
	"sudo systemctl list-units -t service --no-pager --no-legend --all": "list-units-systemctl.out.log",
}

// FetchLogs downloads logs from managed node group instances.
func (ts *tester) FetchLogs() (err error) {
	if !ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ts.cfg.Logger.Info("skipping fetch logs for node groups")
		return nil
	}
	if !ts.cfg.EKSConfig.AddOnManagedNodeGroups.FetchLogs {
		ts.cfg.Logger.Info("skipping fetch logs for node groups")
		return nil
	}

	ts.logsMu.Lock()
	defer ts.logsMu.Unlock()

	err = os.MkdirAll(ts.cfg.EKSConfig.AddOnManagedNodeGroups.LogsDir, 0700)
	if err != nil {
		ts.cfg.Logger.Warn("failed to mkdir", zap.Error(err))
		return err
	}

	err = ts.fetchLogs(150, 10)
	if err != nil {
		ts.cfg.Logger.Warn("failed to fetch logs", zap.Error(err))
		return err
	}

	fpath := filepath.Join(os.TempDir(), ts.cfg.EKSConfig.Name+"-logs-mng.tar.gz")
	err = os.RemoveAll(fpath)
	if err != nil {
		ts.cfg.Logger.Warn("failed to remove temp file", zap.Error(err))
		return err
	}

	ts.cfg.Logger.Info("gzipping logs dir", zap.String("logs-dir", ts.cfg.EKSConfig.AddOnManagedNodeGroups.LogsDir), zap.String("file-path", fpath))
	err = archiver.Archive([]string{ts.cfg.EKSConfig.AddOnManagedNodeGroups.LogsDir}, fpath)
	if err != nil {
		ts.cfg.Logger.Warn("archive failed", zap.Error(err))
		return err
	}
	s, err := os.Stat(fpath)
	if err != nil {
		ts.cfg.Logger.Warn("failed to os stat", zap.Error(err))
		return err
	}
	sz := humanize.Bytes(uint64(s.Size()))
	ts.cfg.Logger.Info("gzipped logs dir", zap.String("logs-dir", ts.cfg.EKSConfig.AddOnManagedNodeGroups.LogsDir), zap.String("file-path", fpath), zap.String("file-size", sz))

	if ts.cfg.EKSConfig.S3BucketName != "" {
		rf, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
		if err != nil {
			ts.cfg.Logger.Warn("failed to read a file", zap.Error(err))
			return err
		}
		defer rf.Close()

		s3Key := path.Join(ts.cfg.EKSConfig.Name, filepath.Base(fpath))
		_, err = ts.cfg.S3API.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(ts.cfg.EKSConfig.S3BucketName),
			Key:    aws.String(s3Key),
			Body:   rf,

			// https://docs.aws.amazon.com/AmazonS3/latest/dev/acl-overview.html#canned-acl
			// vs. "public-read"
			ACL: aws.String("private"),

			Metadata: map[string]*string{
				"Kind": aws.String("aws-k8s-tester"),
			},
		})
		if err == nil {
			ts.cfg.Logger.Info("uploaded the gzipped file",
				zap.String("bucket", ts.cfg.EKSConfig.S3BucketName),
				zap.String("remote-path", s3Key),
				zap.String("file-size", sz),
			)
		} else {
			ts.cfg.Logger.Warn("failed to upload the gzipped file",
				zap.String("bucket", ts.cfg.EKSConfig.S3BucketName),
				zap.String("remote-path", s3Key),
				zap.String("file-size", sz),
				zap.Error(err),
			)
		}
	} else {
		ts.cfg.Logger.Info("skipping S3 uploads")
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) fetchLogs(qps float32, burst int) error {
	logsDir := ts.cfg.EKSConfig.AddOnManagedNodeGroups.LogsDir
	sshOpt := ssh.WithVerbose(ts.cfg.EKSConfig.LogLevel == "debug")
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	rch, waits := make(chan instanceLogs, 10), 0

	for name, nodeGroup := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		ts.cfg.Logger.Info("fetching logs from managed node group",
			zap.String("mng-name", name),
			zap.Int("nodes", len(nodeGroup.Instances)),
		)
		waits += len(nodeGroup.Instances)

		for instID, iv := range nodeGroup.Instances {
			pfx := instID + "-"

			go func(instID, logsDir, pfx string, iv ec2config.Instance) {
				select {
				case <-ts.cfg.Stopc:
					ts.cfg.Logger.Warn("exiting fetch logger", zap.String("prefix", pfx))
					return
				default:
				}

				if !rateLimiter.Allow() {
					ts.cfg.Logger.Debug("waiting for rate limiter before SSH into the machine",
						zap.Float32("qps", qps),
						zap.Int("burst", burst),
						zap.String("instance-id", instID),
					)
					werr := rateLimiter.Wait(context.Background())
					ts.cfg.Logger.Debug("waited for rate limiter",
						zap.Float32("qps", qps),
						zap.Int("burst", burst),
						zap.Error(werr),
					)
				}

				sh, err := ssh.New(ssh.Config{
					Logger:        ts.cfg.Logger,
					KeyPath:       ts.cfg.EKSConfig.RemoteAccessPrivateKeyPath,
					PublicIP:      iv.PublicIP,
					PublicDNSName: iv.PublicDNSName,
					UserName:      iv.RemoteAccessUserName,
				})
				if err != nil {
					rch <- instanceLogs{mngName: name, errs: []string{err.Error()}}
					return
				}
				defer sh.Close()
				if err = sh.Connect(); err != nil {
					rch <- instanceLogs{mngName: name, errs: []string{err.Error()}}
					return
				}

				data := instanceLogs{mngName: name, instanceID: instID}
				// fetch default logs
				for cmd, fileName := range defaultLogs {
					if !rateLimiter.Allow() {
						ts.cfg.Logger.Debug("waiting for rate limiter before fetching file")
						werr := rateLimiter.Wait(context.Background())
						ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
					}
					out, oerr := sh.Run(cmd, sshOpt)
					if oerr != nil {
						data.errs = append(data.errs, fmt.Sprintf("failed to run command %q for %q (error %v)", cmd, instID, oerr))
						continue
					}

					fpath := filepath.Join(logsDir, shorten(ts.cfg.Logger, pfx+fileName))
					f, err := os.Create(fpath)
					if err != nil {
						data.errs = append(data.errs, fmt.Sprintf(
							"failed to create a file %q for %q (error %v)",
							fpath,
							instID,
							err,
						))
						continue
					}
					if _, err = f.Write(out); err != nil {
						data.errs = append(data.errs, fmt.Sprintf(
							"failed to write to a file %q for %q (error %v)",
							fpath,
							instID,
							err,
						))
						f.Close()
						continue
					}
					f.Close()
					ts.cfg.Logger.Debug("wrote", zap.String("file-path", fpath))
					data.paths = append(data.paths, fpath)
				}

				if !rateLimiter.Allow() {
					ts.cfg.Logger.Debug("waiting for rate limiter before fetching file")
					werr := rateLimiter.Wait(context.Background())
					ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
				}
				ts.cfg.Logger.Info("listing systemd service units", zap.String("instance-id", instID))
				listCmd := "sudo systemctl list-units -t service --no-pager --no-legend --all"
				out, oerr := sh.Run(listCmd, sshOpt)
				if oerr != nil {
					data.errs = append(data.errs, fmt.Sprintf(
						"failed to run command %q for %q (error %v)",
						listCmd,
						instID,
						oerr,
					))
				} else {
					/*
						auditd.service                                        loaded    active   running Security Auditing Service
						auth-rpcgss-module.service                            loaded    inactive dead    Kernel Module supporting RPCSEC_GSS
					*/
					svcCmdToFileName := make(map[string]string)
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
						svc := fields[0]
						svcCmd := "sudo journalctl --no-pager --output=cat -u " + svc
						svcFileName := svc + ".out.log"
						svcCmdToFileName[svcCmd] = svcFileName
					}
					for cmd, fileName := range svcCmdToFileName {
						if !rateLimiter.Allow() {
							ts.cfg.Logger.Debug("waiting for rate limiter before fetching file")
							werr := rateLimiter.Wait(context.Background())
							ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
						}
						out, oerr := sh.Run(cmd, sshOpt)
						if oerr != nil {
							data.errs = append(data.errs, fmt.Sprintf(
								"failed to run command %q for %q (error %v)",
								listCmd,
								instID,
								oerr,
							))
							continue
						}

						fpath := filepath.Join(logsDir, shorten(ts.cfg.Logger, pfx+fileName))
						f, err := os.Create(fpath)
						if err != nil {
							data.errs = append(data.errs, fmt.Sprintf(
								"failed to create a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							))
							continue
						}
						if _, err = f.Write(out); err != nil {
							data.errs = append(data.errs, fmt.Sprintf(
								"failed to write to a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							))
							f.Close()
							continue
						}
						f.Close()
						ts.cfg.Logger.Debug("wrote", zap.String("file-path", fpath))
						data.paths = append(data.paths, fpath)
					}
				}

				if !rateLimiter.Allow() {
					ts.cfg.Logger.Debug("waiting for rate limiter before fetching file")
					werr := rateLimiter.Wait(context.Background())
					ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
				}
				// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/docs/troubleshooting.md#ipamd-debugging-commands
				// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/scripts/aws-cni-support.sh
				ts.cfg.Logger.Info("fetching ENI information", zap.String("instance-id", instID))
				eniCmd := "curl -s http://localhost:61679/v1/enis"
				out, oerr = sh.Run(eniCmd, sshOpt)
				if oerr != nil {
					data.errs = append(data.errs, fmt.Sprintf(
						"failed to run command %q for %q (error %v)",
						eniCmd,
						instID,
						oerr,
					))
				} else {
					v1ENIOutputPath := filepath.Join(logsDir, shorten(ts.cfg.Logger, pfx+"v1-enis.out.log"))
					f, err := os.Create(v1ENIOutputPath)
					if err != nil {
						data.errs = append(data.errs, fmt.Sprintf(
							"failed to create a file %q for %q (error %v)",
							v1ENIOutputPath,
							instID,
							err,
						))
					} else {
						if _, err = f.Write(out); err != nil {
							data.errs = append(data.errs, fmt.Sprintf(
								"failed to write to a file %q for %q (error %v)",
								v1ENIOutputPath,
								instID,
								err,
							))
						} else {
							ts.cfg.Logger.Debug("wrote", zap.String("file-path", v1ENIOutputPath))
							data.paths = append(data.paths, v1ENIOutputPath)
						}
						f.Close()
					}
				}

				ts.cfg.Logger.Info("running /opt/cni/bin/aws-cni-support.sh", zap.String("instance-id", instID))
				cniCmd := "sudo /opt/cni/bin/aws-cni-support.sh || true"
				out, oerr = sh.Run(cniCmd, sshOpt)
				if oerr != nil {
					data.errs = append(data.errs, fmt.Sprintf(
						"failed to run command %q for %q (error %v)",
						cniCmd,
						instID,
						oerr,
					))
				} else {
					ts.cfg.Logger.Info("ran /opt/cni/bin/aws-cni-support.sh", zap.String("instance-id", instID), zap.String("output", string(out)))
				}

				if !rateLimiter.Allow() {
					ts.cfg.Logger.Debug("waiting for rate limiter before fetching file")
					werr := rateLimiter.Wait(context.Background())
					ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
				}
				ts.cfg.Logger.Info("listing /var/log", zap.String("instance-id", instID))
				findCmd := "sudo find /var/log ! -type d"
				out, oerr = sh.Run(findCmd, sshOpt)
				if oerr != nil {
					data.errs = append(data.errs, fmt.Sprintf(
						"failed to run command %q for %q (error %v)",
						findCmd,
						instID,
						oerr,
					))
				} else {
					varLogPaths := make(map[string]string)
					for _, line := range strings.Split(string(out), "\n") {
						if len(line) == 0 {
							// last value
							continue
						}
						logCmd := "sudo cat " + line
						logPath := filepath.Base(line)
						varLogPaths[logCmd] = logPath
					}
					for cmd, logPath := range varLogPaths {
						if !rateLimiter.Allow() {
							ts.cfg.Logger.Debug("waiting for rate limiter before fetching file")
							werr := rateLimiter.Wait(context.Background())
							ts.cfg.Logger.Debug("waited for rate limiter", zap.Error(werr))
						}
						out, oerr := sh.Run(cmd, sshOpt)
						if oerr != nil {
							data.errs = append(data.errs, fmt.Sprintf(
								"failed to run command %q for %q (error %v)",
								cmd,
								instID,
								oerr,
							))
							continue
						}

						fpath := filepath.Join(logsDir, shorten(ts.cfg.Logger, pfx+logPath))
						f, err := os.Create(fpath)
						if err != nil {
							data.errs = append(data.errs, fmt.Sprintf(
								"failed to create a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							))
							continue
						}
						if _, err = f.Write(out); err != nil {
							data.errs = append(data.errs, fmt.Sprintf(
								"failed to write to a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							))
							f.Close()
							continue
						}
						f.Close()
						ts.cfg.Logger.Debug("wrote", zap.String("file-path", fpath))
						data.paths = append(data.paths, fpath)
					}
				}
				rch <- data
			}(instID, logsDir, pfx, iv)
		}
	}

	ts.cfg.Logger.Info("waiting for log fetcher goroutines", zap.Int("waits", waits))
	total := 0
	for i := 0; i < waits; i++ {
		var data instanceLogs
		select {
		case data = <-rch:
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("exiting fetch logger")
			return ts.cfg.EKSConfig.Sync()
		}
		if len(data.errs) > 0 {
			ts.cfg.Logger.Warn("failed to fetch logs",
				zap.String("mng-name", data.mngName),
				zap.String("instance-id", data.instanceID),
				zap.Strings("errors", data.errs),
			)
			continue
		}
		mv, ok := ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[data.mngName]
		if !ok {
			return fmt.Errorf("EKS Managed Node Group name %q is unknown", data.mngName)
		}
		if mv.Logs == nil {
			mv.Logs = make(map[string][]string)
		}
		_, ok = mv.Logs[data.instanceID]
		if ok {
			return fmt.Errorf("EKS Managed Node Group name %q for instance %q logs are redundant", data.mngName, data.instanceID)
		}

		mv.Logs[data.instanceID] = data.paths

		ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs[data.mngName] = mv
		ts.cfg.EKSConfig.Sync()

		files := len(data.paths)
		total += files
		ts.cfg.Logger.Info("wrote log files",
			zap.String("instance-id", data.instanceID),
			zap.Int("files", files),
			zap.Int("total-downloaded-files", total),
			zap.Int("total-waits", waits),
			zap.Int("current-wait", i),
		)
	}

	ts.cfg.Logger.Info("wrote all log files",
		zap.String("log-dir", logsDir),
		zap.Int("total-downloaded-files", total),
	)
	return ts.cfg.EKSConfig.Sync()
}

type instanceLogs struct {
	mngName    string
	instanceID string
	paths      []string
	errs       []string
}

func (ts *tester) DownloadClusterLogs(artifactDir string) error {
	err := ts.FetchLogs()
	if err != nil {
		return err
	}

	ts.logsMu.RLock()
	defer ts.logsMu.RUnlock()

	for _, cur := range ts.cfg.EKSConfig.AddOnManagedNodeGroups.MNGs {
		for _, fpaths := range cur.Logs {
			for _, fpath := range fpaths {
				newPath := filepath.Join(artifactDir, filepath.Base(fpath))
				if err := fileutil.Copy(fpath, newPath); err != nil {
					return err
				}
			}
		}
	}

	return fileutil.Copy(
		ts.cfg.EKSConfig.ConfigPath,
		filepath.Join(artifactDir, filepath.Base(ts.cfg.EKSConfig.ConfigPath)),
	)
}

func shorten(lg *zap.Logger, name string) string {
	if len(name) < 240 {
		return name
	}

	ext := filepath.Ext(name)
	oldName := name

	name = name[:230] + randutil.String(5) + ext
	lg.Info("file name too long; renamed", zap.String("old", oldName), zap.String("new", name))
	return name
}
