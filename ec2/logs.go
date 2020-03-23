package ec2

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/ssh"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/dustin/go-humanize"
	"github.com/mholt/archiver/v3"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// TODO: fetch logs via SSM + S3

var logCmds = map[string]string{
	// kernel logs
	"sudo journalctl --no-pager --output=short-precise -k": "kernel.out.log",

	// full journal logs (e.g. disk mounts)
	"sudo journalctl --no-pager --output=short-precise": "journal.out.log",

	// other systemd services
	"sudo systemctl list-units -t service --no-pager --no-legend --all": "list-units-systemctl.out.log",
}

// FetchLogs downloads logs from managed node group instances.
func (ts *Tester) FetchLogs() (err error) {
	if !ts.cfg.ASGsFetchLogs {
		return nil
	}
	if len(ts.cfg.ASGs) == 0 {
		ts.lg.Info("empty ASGs; no logs to fetch")
		return nil
	}

	ts.logsMu.Lock()
	defer ts.logsMu.Unlock()

	err = os.MkdirAll(ts.cfg.ASGsLogsDir, 0700)
	if err != nil {
		ts.lg.Warn("failed to mkdir", zap.Error(err))
		return err
	}

	err = ts.fetchLogs(150, 10, logCmds)
	if err != nil {
		ts.lg.Warn("failed to fetch logs", zap.Error(err))
		return err
	}

	fpath := filepath.Join(os.TempDir(), ts.cfg.Name+"-logs.tar.gz")
	err = os.RemoveAll(fpath)
	if err != nil {
		ts.lg.Warn("failed to remove temp file", zap.Error(err))
		return err
	}

	ts.lg.Info("gzipping logs dir", zap.String("logs-dir", ts.cfg.ASGsLogsDir), zap.String("file-path", fpath))
	err = archiver.Archive([]string{ts.cfg.ASGsLogsDir}, fpath)
	if err != nil {
		ts.lg.Warn("archive failed", zap.Error(err))
		return err
	}
	s, err := os.Stat(fpath)
	if err != nil {
		ts.lg.Warn("failed to os stat", zap.Error(err))
		return err
	}
	sz := humanize.Bytes(uint64(s.Size()))
	ts.lg.Info("gzipped logs dir", zap.String("logs-dir", ts.cfg.ASGsLogsDir), zap.String("file-path", fpath), zap.String("file-size", sz))

	if ts.cfg.S3BucketName != "" {
		rf, err := os.OpenFile(fpath, os.O_RDONLY, 0444)
		if err != nil {
			ts.lg.Warn("failed to read a file", zap.Error(err))
			return err
		}
		defer rf.Close()

		s3Key := path.Join(ts.cfg.Name, filepath.Base(fpath))
		_, err = ts.s3API.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(ts.cfg.S3BucketName),
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
			ts.lg.Info("uploaded the gzipped file",
				zap.String("bucket", ts.cfg.S3BucketName),
				zap.String("remote-path", s3Key),
				zap.String("file-size", sz),
			)
		} else {
			ts.lg.Warn("failed to upload the gzipped file",
				zap.String("bucket", ts.cfg.S3BucketName),
				zap.String("remote-path", s3Key),
				zap.String("file-size", sz),
				zap.Error(err),
			)
		}
	} else {
		ts.lg.Info("skipping S3 uploads")
	}

	return ts.cfg.Sync()
}

// only letters and numbers
var regex = regexp.MustCompile("[^a-zA-Z0-9]+")

func (ts *Tester) fetchLogs(qps float32, burst int, commandToFileName map[string]string) error {
	logsDir := ts.cfg.ASGsLogsDir
	sshOpt := ssh.WithVerbose(ts.cfg.LogLevel == "debug")
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	rch, waits := make(chan instanceLogs, 10), 0

	for name, cur := range ts.cfg.ASGs {
		ts.lg.Info("fetching logs",
			zap.String("asg-name", name),
			zap.Int("instances", len(cur.Instances)),
		)
		waits += len(cur.Instances)

		for instID, iv := range cur.Instances {
			pfx := instID + "-"

			go func(instID, logsDir, pfx string, iv ec2config.Instance) {
				select {
				case <-ts.stopCreationCh:
					ts.lg.Warn("exiting fetch logger", zap.String("prefix", pfx))
					return
				default:
				}

				if !rateLimiter.Allow() {
					ts.lg.Debug("waiting for rate limiter before SSH into the machine",
						zap.Float32("qps", qps),
						zap.Int("burst", burst),
						zap.String("instance-id", instID),
					)
					werr := rateLimiter.Wait(context.Background())
					ts.lg.Debug("waited for rate limiter",
						zap.Float32("qps", qps),
						zap.Int("burst", burst),
						zap.Error(werr),
					)
				}

				sh, err := ssh.New(ssh.Config{
					Logger:        ts.lg,
					KeyPath:       ts.cfg.RemoteAccessPrivateKeyPath,
					PublicIP:      iv.PublicIP,
					PublicDNSName: iv.PublicDNSName,
					UserName:      iv.RemoteAccessUserName,
				})
				if err != nil {
					rch <- instanceLogs{asgName: name, errs: []string{err.Error()}}
					return
				}
				defer sh.Close()
				if err = sh.Connect(); err != nil {
					rch <- instanceLogs{asgName: name, errs: []string{err.Error()}}
					return
				}

				data := instanceLogs{asgName: name, instanceID: instID}
				// fetch default logs
				for cmd, fileName := range commandToFileName {
					if !rateLimiter.Allow() {
						ts.lg.Debug("waiting for rate limiter before fetching file")
						werr := rateLimiter.Wait(context.Background())
						ts.lg.Debug("waited for rate limiter", zap.Error(werr))
					}
					out, oerr := sh.Run(cmd, sshOpt)
					if oerr != nil {
						data.errs = append(data.errs, fmt.Sprintf("failed to run command %q for %q (error %v)", cmd, instID, oerr))
						continue
					}

					fpath := filepath.Join(logsDir, shorten(ts.lg, pfx+fileName))
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
					ts.lg.Debug("wrote", zap.String("file-path", fpath))
					data.paths = append(data.paths, fpath)
				}

				if !rateLimiter.Allow() {
					ts.lg.Debug("waiting for rate limiter before fetching file")
					werr := rateLimiter.Wait(context.Background())
					ts.lg.Debug("waited for rate limiter", zap.Error(werr))
				}
				ts.lg.Info("listing systemd service units", zap.String("instance-id", instID))
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
							ts.lg.Debug("waiting for rate limiter before fetching file")
							werr := rateLimiter.Wait(context.Background())
							ts.lg.Debug("waited for rate limiter", zap.Error(werr))
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

						fpath := filepath.Join(logsDir, shorten(ts.lg, pfx+fileName))
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
						ts.lg.Debug("wrote", zap.String("file-path", fpath))
						data.paths = append(data.paths, fpath)
					}
				}

				if !rateLimiter.Allow() {
					ts.lg.Debug("waiting for rate limiter before fetching file")
					werr := rateLimiter.Wait(context.Background())
					ts.lg.Debug("waited for rate limiter", zap.Error(werr))
				}
				ts.lg.Info("listing /var/log", zap.String("instance-id", instID))
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
					varLogCmdToFileName := make(map[string]string)
					for _, line := range strings.Split(string(out), "\n") {
						if len(line) == 0 {
							// last value
							continue
						}
						logCmd := "sudo cat " + line
						logName := filepath.Base(line)
						varLogCmdToFileName[logCmd] = logName
					}
					for cmd, fileName := range varLogCmdToFileName {
						if !rateLimiter.Allow() {
							ts.lg.Debug("waiting for rate limiter before fetching file")
							werr := rateLimiter.Wait(context.Background())
							ts.lg.Debug("waited for rate limiter", zap.Error(werr))
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

						fpath := filepath.Join(logsDir, shorten(ts.lg, pfx+fileName))
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
						ts.lg.Debug("wrote", zap.String("file-path", fpath))
						data.paths = append(data.paths, fpath)
					}
				}
				rch <- data
			}(instID, logsDir, pfx, iv)
		}
	}

	ts.lg.Info("waiting for log fetcher goroutines", zap.Int("waits", waits))
	total := 0
	for i := 0; i < waits; i++ {
		var data instanceLogs
		select {
		case data = <-rch:
		case <-ts.stopCreationCh:
			ts.lg.Warn("exiting fetch logger")
			return ts.cfg.Sync()
		}
		if len(data.errs) > 0 {
			ts.lg.Warn("failed to fetch logs",
				zap.String("asg-name", data.asgName),
				zap.String("instance-id", data.instanceID),
				zap.Strings("errors", data.errs),
			)
			continue
		}
		mv, ok := ts.cfg.ASGs[data.asgName]
		if !ok {
			return fmt.Errorf("EKS Managed Node Group name %q is unknown", data.asgName)
		}
		if mv.Logs == nil {
			mv.Logs = make(map[string][]string)
		}
		_, ok = mv.Logs[data.instanceID]
		if ok {
			return fmt.Errorf("EKS Managed Node Group name %q for instance %q logs are redundant", data.asgName, data.instanceID)
		}

		mv.Logs[data.instanceID] = data.paths

		ts.cfg.ASGs[data.asgName] = mv
		ts.cfg.Sync()

		files := len(data.paths)
		total += files
		ts.lg.Info("wrote log files",
			zap.String("instance-id", data.instanceID),
			zap.Int("files", files),
			zap.Int("total-downloaded-files", total),
		)
	}

	ts.lg.Info("wrote all log files",
		zap.String("log-dir", logsDir),
		zap.Int("total-downloaded-files", total),
	)
	return ts.cfg.Sync()
}

type instanceLogs struct {
	asgName    string
	instanceID string
	paths      []string
	errs       []string
}

// DownloadLogs downloads logs to the artifact direcoty.
func (ts *Tester) DownloadLogs(artifactDir string) error {
	err := ts.FetchLogs()
	if err != nil {
		return err
	}

	ts.logsMu.RLock()
	defer ts.logsMu.RUnlock()

	for _, v := range ts.cfg.ASGs {
		for _, fpaths := range v.Logs {
			for _, fpath := range fpaths {
				newPath := filepath.Join(artifactDir, filepath.Base(fpath))
				if err := fileutil.Copy(fpath, newPath); err != nil {
					return err
				}
			}
		}
	}

	return fileutil.Copy(
		ts.cfg.ConfigPath,
		filepath.Join(artifactDir, filepath.Base(ts.cfg.ConfigPath)),
	)
}

func shorten(lg *zap.Logger, name string) string {
	if len(name) < 240 {
		return name
	}

	ext := filepath.Ext(name)
	oldName := name

	name = name[:230] + randString(5) + ext
	lg.Info("file name too long; renamed", zap.String("old", oldName), zap.String("new", name))
	return name
}
