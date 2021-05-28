package client

import (
	"context"
	"time"

	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_client "k8s.io/client-go/kubernetes"
)

// ListSecrets returns list of cluster nodes.
func ListSecrets(lg *zap.Logger, cli k8s_client.Interface, namespace string, batchLimit int64, batchInterval time.Duration) (ss []core_v1.Secret, err error) {
	lg.Info("listing secrets",
		zap.String("namespace", namespace),
		zap.Int64("batch-limit", batchLimit),
		zap.Duration("batch-interval", batchInterval),
	)
	rs := &core_v1.SecretList{ListMeta: meta_v1.ListMeta{Continue: ""}}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		rs, err = cli.CoreV1().Secrets(namespace).List(ctx, meta_v1.ListOptions{Limit: batchLimit, Continue: rs.Continue})
		cancel()
		if err != nil {
			return nil, err
		}
		ss = append(ss, rs.Items...)
		remained := int64Value(rs.RemainingItemCount)
		lg.Info("secrets",
			zap.Int64("remained", remained),
			zap.Int("items", len(rs.Items)),
		)
		if rs.Continue == "" {
			break
		}
		time.Sleep(batchInterval)
	}
	lg.Info("listed secrets", zap.Int("secrets", len(ss)))
	return ss, err
}
