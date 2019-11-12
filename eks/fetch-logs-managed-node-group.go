package eks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-k8s-tester/ec2config"
	"github.com/aws/aws-k8s-tester/internal/ssh"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var logCommandsForManagedNodeGroup = map[string]string{
	// kernel logs
	"sudo journalctl --no-pager --output=short-precise -k": "kernel.out.log",

	// full journal logs (e.g. disk mounts)
	"sudo journalctl --no-pager --output=short-precise": "journal.out.log",

	// other systemd services
	"sudo systemctl list-units -t service --no-pager --no-legend --all": "list-units-systemctl.out.log",
}

// FetchLogsManagedNodeGroup downloads logs from managed node group instances.
func (ts *Tester) FetchLogsManagedNodeGroup() (err error) {
	ts.fetchLogsManagedNodeGroupMu.Lock()
	defer ts.fetchLogsManagedNodeGroupMu.Unlock()
	return ts.fetchLogsManagedNodeGroup(300, 50, logCommandsForManagedNodeGroup)
}

func (ts *Tester) fetchLogsManagedNodeGroup(qps float32, burst int, commandToFileName map[string]string) error {
	logsDir, err := ioutil.TempDir(filepath.Dir(ts.cfg.ConfigPath), ts.cfg.Name+"-managed-node-group-logs")
	if err != nil {
		return err
	}

	rateLimiter := rate.NewLimiter(rate.Limit(qps), burst)
	rch, waits := make(chan instanceLogs, 10), 0

	for asgName, nodeGroup := range ts.cfg.Status.ManagedNodeGroups {
		ts.lg.Info("fetching logs",
			zap.String("asg-name", asgName),
			zap.Int("nodes", len(nodeGroup.Instances)),
		)
		waits += len(nodeGroup.Instances)
		for instID, iv := range nodeGroup.Instances {
			pfx := instID + "-" + iv.PublicDNSName + "-"
			go func(instID, logsDir, pfx string, iv ec2config.Instance) {
				select {
				case <-ts.stopCreationCh:
					ts.lg.Warn("exiting fetch logger", zap.String("prefix", pfx))
					return
				default:
				}
				if !rateLimiter.Allow() {
					ts.lg.Warn("waiting for rate limiter before SSH into the machine",
						zap.Float32("qps", qps),
						zap.Int("burst", burst),
						zap.String("instance-id", instID),
					)
					werr := rateLimiter.Wait(context.Background())
					ts.lg.Warn("waited for rate limiter",
						zap.Float32("qps", qps),
						zap.Int("burst", burst),
						zap.Error(werr),
					)
				}
				sh, err := ssh.New(ssh.Config{
					Logger:        ts.lg,
					KeyPath:       ts.cfg.Parameters.ManagedNodeGroupRemoteAccessPrivateKeyPath,
					PublicIP:      iv.PublicIP,
					PublicDNSName: iv.PublicDNSName,
					UserName:      ts.cfg.Parameters.ManagedNodeGroupRemoteAccessUserName,
				})
				if err != nil {
					rch <- instanceLogs{err: err}
					return
				}
				defer sh.Close()
				if err = sh.Connect(); err != nil {
					rch <- instanceLogs{err: err}
					return
				}

				data := instanceLogs{instanceID: instID}

				// fetch default logs
				for cmd, fileName := range commandToFileName {
					if !rateLimiter.Allow() {
						ts.lg.Warn("waiting for rate limiter before fetching file")
						werr := rateLimiter.Wait(context.Background())
						ts.lg.Warn("waited for rate limiter", zap.Error(werr))
					}
					out, oerr := sh.Run(cmd, ssh.WithVerbose(ts.cfg.LogLevel == "debug"))
					if oerr != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to run command %q for %q (error %v)",
								cmd,
								instID,
								oerr,
							)}
						return
					}
					fpath := filepath.Join(logsDir, pfx+fileName)
					f, err := os.Create(fpath)
					if err != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to create a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							)}
						return
					}
					if _, err = f.Write(out); err != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to write to a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							)}
						f.Close()
						return
					}
					f.Close()
					ts.lg.Debug("wrote", zap.String("file-path", fpath))
					data.paths = append(data.paths, fpath)
				}

				if !rateLimiter.Allow() {
					ts.lg.Warn("waiting for rate limiter before fetching file")
					werr := rateLimiter.Wait(context.Background())
					ts.lg.Warn("waited for rate limiter", zap.Error(werr))
				}
				ts.lg.Info("listing systemd service units", zap.String("instance-id", instID))
				listCmd := "sudo systemctl list-units -t service --no-pager --no-legend --all"
				out, oerr := sh.Run(listCmd, ssh.WithVerbose(ts.cfg.LogLevel == "debug"))
				if oerr != nil {
					rch <- instanceLogs{
						instanceID: instID,
						err: fmt.Errorf(
							"failed to run command %q for %q (error %v)",
							listCmd,
							instID,
							oerr,
						)}
					return
				}
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
						ts.lg.Warn("waiting for rate limiter before fetching file")
						werr := rateLimiter.Wait(context.Background())
						ts.lg.Warn("waited for rate limiter", zap.Error(werr))
					}
					out, oerr := sh.Run(cmd, ssh.WithVerbose(ts.cfg.LogLevel == "debug"))
					if oerr != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to run command %q for %q (error %v)",
								cmd,
								instID,
								oerr,
							)}
						return
					}
					fpath := filepath.Join(logsDir, pfx+fileName)
					f, err := os.Create(fpath)
					if err != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to create a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							)}
						return
					}
					if _, err = f.Write(out); err != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to write to a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							)}
						f.Close()
						return
					}
					f.Close()
					ts.lg.Debug("wrote", zap.String("file-path", fpath))
					data.paths = append(data.paths, fpath)
				}

				if !rateLimiter.Allow() {
					ts.lg.Warn("waiting for rate limiter before fetching file")
					werr := rateLimiter.Wait(context.Background())
					ts.lg.Warn("waited for rate limiter", zap.Error(werr))
				}
				ts.lg.Info("listing /var/log", zap.String("instance-id", instID))
				findCmd := "sudo find /var/log ! -type d"
				out, oerr = sh.Run(findCmd, ssh.WithVerbose(ts.cfg.LogLevel == "debug"))
				if oerr != nil {
					rch <- instanceLogs{
						instanceID: instID,
						err: fmt.Errorf(
							"failed to run command %q for %q (error %v)",
							findCmd,
							instID,
							oerr,
						)}
					return
				}
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
						ts.lg.Warn("waiting for rate limiter before fetching file")
						werr := rateLimiter.Wait(context.Background())
						ts.lg.Warn("waited for rate limiter", zap.Error(werr))
					}
					out, oerr := sh.Run(cmd, ssh.WithVerbose(ts.cfg.LogLevel == "debug"))
					if oerr != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to run command %q for %q (error %v)",
								cmd,
								instID,
								oerr,
							)}
						return
					}
					fpath := filepath.Join(logsDir, pfx+fileName)
					f, err := os.Create(fpath)
					if err != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to create a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							)}
						return
					}
					if _, err = f.Write(out); err != nil {
						rch <- instanceLogs{
							instanceID: instID,
							err: fmt.Errorf(
								"failed to write to a file %q for %q (error %v)",
								fpath,
								instID,
								err,
							)}
						f.Close()
						return
					}
					f.Close()
					ts.lg.Debug("wrote", zap.String("file-path", fpath))
					data.paths = append(data.paths, fpath)
				}

				rch <- data
			}(instID, logsDir, pfx, iv)
		}
	}

	total := 0
	instanceIDToFpaths := make(map[string][]string)
	for i := 0; i < waits; i++ {
		var isl instanceLogs
		select {
		case isl = <-rch:
		case <-ts.stopCreationCh:
			ts.lg.Warn("exiting fetch logger")
			return ts.cfg.Sync()
		}
		if isl.err != nil {
			ts.lg.Error("failed to fetch logs",
				zap.String("instance-id", isl.instanceID),
				zap.Error(isl.err),
			)
			continue
		}
		instanceIDToFpaths[isl.instanceID] = isl.paths
		total += len(isl.paths)
	}

	ts.cfg.Status.ManagedNodeGroupsLogs = instanceIDToFpaths
	ts.lg.Info("wrote all log files", zap.String("log-dir", logsDir), zap.Int("total-files", total))

	return ts.cfg.Sync()
}

type instanceLogs struct {
	instanceID string
	paths      []string
	err        error
}
