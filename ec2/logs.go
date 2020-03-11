package ec2

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-k8s-tester/ssh"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

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
	if err := os.MkdirAll(ts.cfg.ASGsLogsDir, 0700); err != nil {
		return err
	}
	ts.logsMu.Lock()
	defer ts.logsMu.Unlock()
	return ts.fetchLogs(150, 10, logCmds)
}

// only letters and numbers
var regex = regexp.MustCompile("[^a-zA-Z0-9]+")

func (ts *Tester) fetchLogs(qps float32, burst int, commandToFileName map[string]string) error {
	logsDir := ts.cfg.ASGsLogsDir
	sshOpt := ssh.WithVerbose(ts.cfg.LogLevel == "debug")
	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	rch, waits := make(chan instanceLogs, 10), 0

	for name, asg := range ts.cfg.ASGs {
		ts.lg.Info("fetching logs",
			zap.String("asg-name", name),
			zap.Int("instances", len(asg.Instances)),
		)
		waits += len(asg.Instances)

		for instID, iv := range asg.Instances {
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
					UserName:      ts.cfg.RemoteAccessUserName,
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
				// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/docs/troubleshooting.md#ipamd-debugging-commands
				// https://github.com/aws/amazon-vpc-cni-k8s/blob/master/scripts/aws-cni-support.sh
				ts.lg.Info("fetching ENI information", zap.String("instance-id", instID))
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
					v1ENIOutputPath := filepath.Join(logsDir, shorten(ts.lg, pfx+"v1-enis.out.log"))
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
							ts.lg.Debug("wrote", zap.String("file-path", v1ENIOutputPath))
							data.paths = append(data.paths, v1ENIOutputPath)
						}
						f.Close()
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
				zap.String("mng-name", data.mngName),
				zap.String("instance-id", data.instanceID),
				zap.Strings("errors", data.errs),
			)
			continue
		}
		mv, ok := ts.cfg.ASGs[data.mngName]
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

		ts.cfg.ASGs[data.mngName] = mv
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
	mngName    string
	instanceID string
	paths      []string
	errs       []string
}

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

const ll = "0123456789abcdefghijklmnopqrstuvwxyz"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		rand.Seed(time.Now().UnixNano())
		b[i] = ll[rand.Intn(len(ll))]
	}
	return string(b)
}
