// Package ssh implements various SSH commands.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
	"k8s.io/utils/exec"
)

// Config defines SSH configuration.
type Config struct {
	Logger  *zap.Logger
	KeyPath string

	PublicIP      string
	PublicDNSName string

	// UserName is the user name to use for log-in.
	// "ec2-user" for Amazon Linux 2
	// "ubuntu" for ubuntu
	UserName string

	// Envs is the set of environmental variables to use
	// in the SSH session.
	Envs map[string]string
}

// SSH defines SSH operations.
// For example, automates the following:
//
//  ssh -o "StrictHostKeyChecking no" \
//    -i ./aws-k8s-tester-ec2.key669686897 \
//    ec2-user@ec2-35-166-71-150.us-west-2.compute.amazonaws.com
//
//  rm -f ./text.txt
//  echo "Hello" > ./text.txt
//
//  scp -oStrictHostKeyChecking=no \
//    -i ./aws-k8s-tester-ec2.key301005900 \
//    ./text.txt \
//    ec2-user@ec2-35-166-71-150.us-west-2.compute.amazonaws.com:/home/ec2-user/test.txt
//
//  scp -oStrictHostKeyChecking=no \
//    -i ./aws-k8s-tester-ec2.key301005900 \
//    ./testfile449686843 \
//    ec2-user@34.220.64.30:22:/home/ec2-user/aws-k8s-tester.txt
//
//  scp -oStrictHostKeyChecking=no \
//    -i ./aws-k8s-tester-ec2.key301005900 \
//    ec2-user@ec2-35-166-71-150.us-west-2.compute.amazonaws.com:/home/ec2-user/test.txt \
//    ./test2.txt
//
type SSH interface {
	// Connect connects to a remote server creating a new client session.
	// "Close" must be called after use.
	Connect() error
	// Close closes the session and connection to a remote server.
	Close()
	// Run runs the command and returns the output.
	Run(cmd string, opts ...OpOption) (out []byte, err error)
	// Send sends a file to the remote host using SCP protocol.
	Send(localPath, remotePath string, opts ...OpOption) (out []byte, err error)
	// Download downloads a file from the remote host using SCP protocol.
	Download(remotePath, localPath string, opts ...OpOption) (out []byte, err error)
}

type ssh struct {
	cfg Config

	lg *zap.Logger

	key    []byte
	signer cryptossh.Signer

	ctx    context.Context
	cancel context.CancelFunc

	conn net.Conn
	cli  *cryptossh.Client

	// retry counter per instance + command
	retryCounter map[string]int
}

// New returns a new SSH.
func New(cfg Config) (s SSH, err error) {
	sh := &ssh{
		cfg:          cfg,
		lg:           cfg.Logger,
		retryCounter: make(map[string]int),
	}
	if sh.lg == nil {
		sh.lg = zap.NewNop()
	}
	return sh, nil
}

func (sh *ssh) Connect() (err error) {
	sh.ctx, sh.cancel = context.WithCancel(context.Background())
	sh.key, err = ioutil.ReadFile(sh.cfg.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key %v", err)
	}
	sh.signer, err = cryptossh.ParsePrivateKey(sh.key)
	if err != nil {
		return fmt.Errorf("failed to parse private key %v", err)
	}

	var (
		c     cryptossh.Conn
		chans <-chan cryptossh.NewChannel
		reqs  <-chan *cryptossh.Request
	)
	for i := 0; i < 15; i++ {
		select {
		case <-sh.ctx.Done():
			return errors.New("stopped")
		default:
		}

		sh.lg.Debug("dialing",
			zap.String("public-ip", sh.cfg.PublicIP),
			zap.String("public-dns-name", sh.cfg.PublicDNSName),
		)
		d := net.Dialer{}
		ctx, cancel := context.WithTimeout(sh.ctx, 15*time.Second)
		sh.conn, err = d.DialContext(ctx, "tcp", sh.cfg.PublicIP+":22")
		cancel()
		if err != nil {
			oerr, ok := err.(*net.OpError)
			if ok {
				// connect: connection refused
				if strings.Contains(oerr.Err.Error(), syscall.ECONNREFUSED.Error()) {
					sh.lg.Warn(
						"failed to dial (instance might not be ready yet)",
						zap.String("public-ip", sh.cfg.PublicIP),
						zap.String("public-dns-name", sh.cfg.PublicDNSName),
						zap.Error(err),
					)
				}
			} else {
				sh.lg.Warn(
					"failed to dial",
					zap.String("public-ip", sh.cfg.PublicIP),
					zap.String("public-dns-name", sh.cfg.PublicDNSName),
					zap.String("error-type", fmt.Sprintf("%v", reflect.TypeOf(err))),
					zap.Error(err),
				)
			}
			time.Sleep(5 * time.Second)
			continue
		}
		sh.lg.Info("dialed",
			zap.String("public-ip", sh.cfg.PublicIP),
			zap.String("public-dns-name", sh.cfg.PublicDNSName),
		)

		sshConfig := &cryptossh.ClientConfig{
			User: sh.cfg.UserName,
			Auth: []cryptossh.AuthMethod{
				cryptossh.PublicKeys(sh.signer),
			},
			HostKeyCallback: cryptossh.InsecureIgnoreHostKey(),
		}
		c, chans, reqs, err = cryptossh.NewClientConn(sh.conn, sh.cfg.PublicIP+":22", sshConfig)
		if err != nil {
			fi, _ := os.Stat(sh.cfg.KeyPath)
			sh.lg.Warn(
				"failed to connect",
				zap.String("public-ip", sh.cfg.PublicIP),
				zap.String("public-dns-name", sh.cfg.PublicDNSName),
				zap.String("file-mode", fi.Mode().String()),
				zap.String("error-type", fmt.Sprintf("%v", reflect.TypeOf(err))),
				zap.Error(err),
			)
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}
	if err != nil {
		return err
	}

	sh.cli = cryptossh.NewClient(c, chans, reqs)
	sh.lg.Debug("created client",
		zap.String("public-ip", sh.cfg.PublicIP),
		zap.String("public-dns-name", sh.cfg.PublicDNSName),
	)
	return nil
}

func (sh *ssh) Close() {
	sh.cancel()
	if sh.conn != nil {
		cerr := sh.conn.Close()
		if cerr != nil {
			sh.lg.Warn("closed connection with error",
				zap.String("public-ip", sh.cfg.PublicIP),
				zap.String("public-dns-name", sh.cfg.PublicDNSName),
				zap.Error(cerr),
			)
			return
		}
	}
	sh.lg.Debug("closed connection",
		zap.String("public-ip", sh.cfg.PublicIP),
		zap.String("public-dns-name", sh.cfg.PublicDNSName),
	)
}

func (sh *ssh) Run(cmd string, opts ...OpOption) (out []byte, err error) {
	ret := Op{verbose: false, retriesLeft: 0, retryInterval: time.Duration(0), timeout: 0, envs: make(map[string]string)}
	ret.applyOpts(opts)

	key := fmt.Sprintf("%s%s", sh.cfg.PublicDNSName, cmd)
	if _, ok := sh.retryCounter[key]; !ok {
		sh.retryCounter[key] = ret.retriesLeft
	}

	now := time.Now()

	// session only accepts one call to Run, Start, Shell, Output, or CombinedOutput
	var ss *cryptossh.Session
	ss, err = sh.cli.NewSession()
	if err != nil {
		return nil, err
	}
	ss.Stderr = nil
	ss.Stdout = nil
	if ret.verbose {
		sh.lg.Info("created client session, running command", zap.String("cmd", cmd))
	}

	if len(sh.cfg.Envs) > 0 {
		for k, v := range sh.cfg.Envs {
			if err = ss.Setenv(k, v); err != nil {
				return nil, err
			}
		}
	}
	if len(ret.envs) > 0 {
		for k, v := range ret.envs {
			if err = ss.Setenv(k, v); err != nil {
				return nil, err
			}
		}
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if ret.timeout == 0 {
		ctx, cancel = context.WithCancel(sh.ctx)
	} else {
		ctx, cancel = context.WithTimeout(sh.ctx, ret.timeout)
	}

	donec := make(chan error)
	go func() {
		out, err = ss.CombinedOutput(cmd)
		close(donec)
	}()
	select {
	case <-ctx.Done():
		ss.Close()
		cancel()
		<-donec
		out, err = nil, ctx.Err()
	case <-donec:
		ss.Close()
		cancel()
	}

	if ret.verbose {
		sh.lg.Info("ran command",
			zap.String("cmd", cmd),
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
	}

	if err != nil {
		shouldRetry := true
		oerr, ok := err.(*net.OpError)
		if ok {
			shouldRetry = oerr.Temporary()
			sh.lg.Warn("command run failed",
				zap.String("cmd", cmd),
				zap.Bool("op-error-temporary", oerr.Temporary()),
				zap.Bool("op-error-timeout", oerr.Timeout()),
				zap.Error(err),
			)
		} else {
			if strings.Contains(err.Error(), "exited with status ") {
				shouldRetry = false
			}
			serr, ok := err.(*cryptossh.ExitError)
			if ok {
				shouldRetry = false
				sh.lg.Warn("command run failed with exit code",
					zap.String("cmd", cmd),
					zap.String("error-type", reflect.TypeOf(err).String()),
					zap.Bool("should-retry", shouldRetry),
					zap.Int("exit-code", serr.ExitStatus()),
					zap.Error(err),
				)
			} else {
				sh.lg.Warn("command run failed",
					zap.String("cmd", cmd),
					zap.String("error-type", reflect.TypeOf(err).String()),
					zap.Bool("should-retry", shouldRetry),
					zap.Error(err),
				)
			}
		}

		if shouldRetry && sh.retryCounter[key] > 0 {
			// e.g. "read tcp 10.119.223.210:58688->54.184.39.156:22: read: connection timed out"
			sh.lg.Warn("retrying command run", zap.Int("retries", sh.retryCounter[key]))
			sh.Close()
			for {
				sh.retryCounter[key]--
				if connErr := sh.Connect(); connErr == nil {
					break
				}
				time.Sleep(3 * time.Second)
			}
			time.Sleep(ret.retryInterval)

			// recursively retry
			out, err = sh.Run(cmd, opts...)
		}
	}
	if err == nil {
		delete(sh.retryCounter, key)
	}
	return out, err
}

/*
chmod 400 ./aws-k8s-tester-ec2.key301005900

ssh -o "StrictHostKeyChecking no" \
  -i ./aws-k8s-tester-ec2.key669686897 \
  ec2-user@ec2-35-166-71-150.us-west-2.compute.amazonaws.com

rm -f ./text.txt
echo "Hello" > ./text.txt

scp -oStrictHostKeyChecking=no \
  -i ./aws-k8s-tester-ec2.key301005900 \
  ./text.txt \
  ec2-user@ec2-35-166-71-150.us-west-2.compute.amazonaws.com:/home/ec2-user/test.txt


/usr/bin/scp -oStrictHostKeyChecking=no \
  -i ./aws-k8s-tester-ec2.key301005900 \
  ./testfile449686843 \
  ec2-user@34.220.64.30:22:/home/ec2-user/aws-k8s-tester.txt

scp -oStrictHostKeyChecking=no \
  -i ./aws-k8s-tester-ec2.key301005900 \
  ec2-user@ec2-35-166-71-150.us-west-2.compute.amazonaws.com:/home/ec2-user/test.txt \
  ./test2.txt
*/

func (sh *ssh) Send(localPath, remotePath string, opts ...OpOption) (out []byte, err error) {
	ret := Op{verbose: false, retriesLeft: 0, retryInterval: time.Duration(0), timeout: 0, envs: make(map[string]string)}
	ret.applyOpts(opts)

	key := fmt.Sprintf("%s%s", sh.cfg.PublicDNSName, localPath)
	if _, ok := sh.retryCounter[key]; !ok {
		sh.retryCounter[key] = ret.retriesLeft
	}

	now := time.Now()

	var ctx context.Context
	var cancel context.CancelFunc
	if ret.timeout == 0 {
		ctx, cancel = context.WithCancel(sh.ctx)
	} else {
		ctx, cancel = context.WithTimeout(sh.ctx, ret.timeout)
	}

	scpCmd := exec.New()
	var scpPath string
	scpPath, err = scpCmd.LookPath("scp")
	if err != nil {
		cancel()
		return nil, err
	}
	if err = os.Chmod(sh.cfg.KeyPath, 0400); err != nil {
		cancel()
		return nil, err
	}

	scpArgs := []string{
		scpPath,
		"-oStrictHostKeyChecking=no",
		"-i", sh.cfg.KeyPath,
		localPath,
		fmt.Sprintf("%s@%s:%s", sh.cfg.UserName, sh.cfg.PublicDNSName, remotePath),
	}
	cmd := scpCmd.CommandContext(ctx, scpArgs[0], scpArgs[1:]...)
	out, err = cmd.CombinedOutput()
	for i := 0; i < 3; i++ {
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "Process exited with status") {
			break
		}

		time.Sleep(2 * time.Second)
		sh.lg.Warn("retrying SCP for send", zap.String("cmd", strings.Join(scpArgs, " ")), zap.Error(err))
		out, err = cmd.CombinedOutput()
	}
	cancel()

	fi, ferr := os.Stat(localPath)
	if ferr == nil {
		if ret.verbose {
			sh.lg.Info("sent",
				zap.String("size", humanize.Bytes(uint64(fi.Size()))),
				zap.String("output", string(out)),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
		}
	} else {
		sh.lg.Warn("failed to send",
			zap.String("output", string(out)),
			zap.Error(ferr),
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
	}

	if err != nil {
		oerr, ok := err.(*net.OpError)
		if ok {
			sh.lg.Warn("command scp send failed", zap.Bool("op-error-temporary", oerr.Temporary()), zap.Bool("op-error-timeout", oerr.Timeout()), zap.Error(err))
		} else {
			sh.lg.Warn("command scp send failed", zap.String("error-type", reflect.TypeOf(err).String()), zap.Error(err))
		}
		if sh.retryCounter[key] > 0 {
			sh.lg.Warn("retrying scp send", zap.Int("retries", sh.retryCounter[key]))
			sh.Close()
			for {
				sh.retryCounter[key]--
				if connErr := sh.Connect(); connErr == nil {
					break
				}
				time.Sleep(3 * time.Second)
			}
			time.Sleep(ret.retryInterval)

			// recursively retry
			out, err = sh.Send(localPath, remotePath, opts...)
		}
	}
	if err == nil {
		delete(sh.retryCounter, key)
	}
	return out, err
}

func (sh *ssh) Download(remotePath, localPath string, opts ...OpOption) (out []byte, err error) {
	ret := Op{verbose: false, retriesLeft: 0, retryInterval: time.Duration(0), timeout: 0, envs: make(map[string]string)}
	ret.applyOpts(opts)

	key := fmt.Sprintf("%s%s", sh.cfg.PublicDNSName, localPath)
	if _, ok := sh.retryCounter[key]; !ok {
		sh.retryCounter[key] = ret.retriesLeft
	}

	now := time.Now()

	var ctx context.Context
	var cancel context.CancelFunc
	if ret.timeout == 0 {
		ctx, cancel = context.WithCancel(sh.ctx)
	} else {
		ctx, cancel = context.WithTimeout(sh.ctx, ret.timeout)
	}

	scpCmd := exec.New()
	var scpPath string
	scpPath, err = scpCmd.LookPath("scp")
	if err != nil {
		cancel()
		return nil, err
	}
	if err = os.Chmod(sh.cfg.KeyPath, 0400); err != nil {
		cancel()
		return nil, err
	}
	scpArgs := []string{
		scpPath,
		"-oStrictHostKeyChecking=no",
		"-i", sh.cfg.KeyPath,
		fmt.Sprintf("%s@%s:%s", sh.cfg.UserName, sh.cfg.PublicDNSName, remotePath),
		localPath,
	}
	cmd := scpCmd.CommandContext(ctx, scpArgs[0], scpArgs[1:]...)
	out, err = cmd.CombinedOutput()
	for i := 0; i < 3; i++ {
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "Process exited with status") {
			break
		}

		time.Sleep(2 * time.Second)
		sh.lg.Warn("retrying SCP for download", zap.String("cmd", strings.Join(scpArgs, " ")), zap.Error(err))
		out, err = cmd.CombinedOutput()
	}
	cancel()

	fi, ferr := os.Stat(localPath)
	if ferr == nil {
		if ret.verbose {
			sh.lg.Info("downloaded",
				zap.String("size", humanize.Bytes(uint64(fi.Size()))),
				zap.String("output", string(out)),
				zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
			)
		}
	} else {
		sh.lg.Warn("failed to download",
			zap.String("output", string(out)),
			zap.Error(ferr),
			zap.String("started", humanize.RelTime(now, time.Now(), "ago", "from now")),
		)
	}

	if err != nil {
		oerr, ok := err.(*net.OpError)
		if ok {
			sh.lg.Warn("command scp download failed", zap.Bool("op-error-temporary", oerr.Temporary()), zap.Bool("op-error-timeout", oerr.Timeout()), zap.Error(err))
		} else {
			sh.lg.Warn("command scp download failed", zap.String("error-type", reflect.TypeOf(err).String()), zap.Error(err))
		}
		if sh.retryCounter[key] > 0 {
			sh.lg.Warn("retrying scp download", zap.Int("retries", sh.retryCounter[key]))
			sh.Close()
			for {
				sh.retryCounter[key]--
				if connErr := sh.Connect(); connErr == nil {
					break
				}
				time.Sleep(3 * time.Second)
			}
			time.Sleep(ret.retryInterval)

			// recursively retry
			out, err = sh.Download(remotePath, localPath, opts...)
		}
	}
	if err == nil {
		delete(sh.retryCounter, key)
	}
	return out, err
}

// Op represents a SSH operation.
type Op struct {
	verbose       bool
	retriesLeft   int
	retryInterval time.Duration
	timeout       time.Duration
	envs          map[string]string
}

// OpOption configures archiver operations.
type OpOption func(*Op)

// WithVerbose configures verbose level in SSH operations.
func WithVerbose(b bool) OpOption {
	return func(op *Op) { op.verbose = b }
}

// WithRetry automatically retries the command on closed TCP connection error.
// (e.g. retry immutable operation).
// WithRetry(-1) to retry forever until success.
// e.g. "read tcp 10.119.223.210:58688->54.184.39.156:22: read: connection timed out"
func WithRetry(retries int, interval time.Duration) OpOption {
	return func(op *Op) {
		op.retriesLeft = retries
		op.retryInterval = interval
	}
}

// WithTimeout configures timeout for command run.
func WithTimeout(timeout time.Duration) OpOption {
	return func(op *Op) { op.timeout = timeout }
}

// WithEnv adds an environment variable that will be applied to any
// command executed by Shell or Run. It overwrites the ones set by
// "*ssh.Session.Setenv".
func WithEnv(k, v string) OpOption {
	return func(op *Op) { op.envs[k] = v }
}

func (op *Op) applyOpts(opts []OpOption) {
	for _, opt := range opts {
		opt(op)
	}
}
