// Package etcdclient implements etcd client utilities.
package etcdclient

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/logutil"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"go.uber.org/zap"
)

// Config defines etcd client config.
type Config struct {
	Logger           *zap.Logger
	EtcdClientConfig clientv3.Config
	ListBath         int64
	ListInterval     time.Duration
}

type etcd struct {
	cfg Config
	cli *clientv3.Client
}

func New(cfg Config) (Etcd, error) {
	if cfg.Logger == nil {
		var err error
		cfg.Logger, err = logutil.GetDefaultZapLogger()
		if err != nil {
			return nil, err
		}
	}
	if cfg.EtcdClientConfig.LogConfig == nil {
		lcfg := logutil.GetDefaultZapLoggerConfig()
		cfg.EtcdClientConfig.LogConfig = &lcfg
	}
	cli, err := clientv3.New(cfg.EtcdClientConfig)
	if err != nil {
		return nil, err
	}
	return &etcd{cfg: cfg, cli: cli}, nil
}

// Etcd defines etcd client operations.
type Etcd interface {
	Put(timeout time.Duration, k, v string) error
	Get(timeout time.Duration, k string) ([]*mvccpb.KeyValue, error)
	Campaign(pfx string, timeout time.Duration) (ok bool, err error)
	List(pfx string, listBatch int64, listInterval time.Duration) (rs []*mvccpb.KeyValue, err error)
	Close()
}

func (e *etcd) Close() {
	e.cfg.Logger.Info("closed client", zap.Error(e.cli.Close()))
}

func (e *etcd) Put(timeout time.Duration, k, v string) error {
	e.cfg.Logger.Info("writing", zap.String("key", k))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err := e.cli.Put(ctx, k, v)
	cancel()
	e.cfg.Logger.Info("wrote", zap.String("key", k), zap.Error(err))
	return err
}

func (e *etcd) Get(timeout time.Duration, k string) ([]*mvccpb.KeyValue, error) {
	e.cfg.Logger.Info("getting", zap.String("key", k))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	resp, err := e.cli.Get(ctx, k)
	cancel()
	if err != nil {
		return nil, err
	}
	e.cfg.Logger.Info("got", zap.String("key", k), zap.Int("kvs", len(resp.Kvs)), zap.Error(err))
	return resp.Kvs, err
}

func (e *etcd) Campaign(pfx string, timeout time.Duration) (ok bool, err error) {
	s, err := concurrency.NewSession(e.cli)
	if err != nil {
		return false, err
	}
	defer s.Close()

	ev := concurrency.NewElection(s, pfx)

	e.cfg.Logger.Info("campaigning", zap.String("prefix", pfx))
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	err = ev.Campaign(ctx, "hello")
	cancel()
	if err == nil {
		e.cfg.Logger.Info("elected as a leader")
	} else {
		e.cfg.Logger.Warn("failed to campaign", zap.Error(err))
	}
	return err == nil, nil
}

func (e *etcd) List(pfx string, listBatch int64, listInterval time.Duration) (rs []*mvccpb.KeyValue, err error) {
	if listBatch == 0 {
		return nil, fmt.Errorf("invalid list batch limit %d", listBatch)
	}
	// see "k8s.io/apiserver/pkg/storage/etcd3" to see how kube-apiserver paginates
	// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apiserver/pkg/storage/etcd3/store.go
	opts := []clientv3.OpOption{
		clientv3.WithRange(clientv3.GetPrefixRangeEnd(pfx)),
		clientv3.WithLimit(listBatch),
	}
	key, resp := pfx, &clientv3.GetResponse{More: true}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		resp, err = e.cli.Get(ctx, key, opts...)
		cancel()
		if err != nil {
			return nil, err
		}
		e.cfg.Logger.Info("getting response", zap.String("start-key", key), zap.Int("kvs", len(resp.Kvs)), zap.Bool("more", resp.More))
		if len(resp.Kvs) == 0 {
			break
		}
		rs = append(rs, resp.Kvs...)
		if !resp.More {
			break
		}

		lastKey := resp.Kvs[len(resp.Kvs)-1].Key
		key = string(lastKey) + "\x00"

		time.Sleep(e.cfg.ListInterval)
	}
	e.cfg.Logger.Info("got response", zap.Int("kvs", len(rs)))
	return rs, err
}
