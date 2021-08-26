package client

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	storage_v1 "k8s.io/api/storage/v1"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_client "k8s.io/client-go/kubernetes"
)

func ListStorageClass(
	lg *zap.Logger,
	c k8s_client.Interface,
	batchLimit int64,
	batchInterval time.Duration,
	opts ...OpOption) (storageclass []storage_v1.StorageClass, err error) {
	ns, err := listStorageClass(lg, c, batchLimit, batchInterval, 5, opts...)
	return ns, err
}

func listStorageClass(
	lg *zap.Logger,
	c k8s_client.Interface,
	batchLimit int64,
	batchInterval time.Duration,
	retryLeft int,
	opts ...OpOption) (storageclass []storage_v1.StorageClass, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	lg.Info("listing storageclass",
		zap.Int64("batch-limit", batchLimit),
		zap.Duration("batch-interval", batchInterval),
		zap.String("label-selector", ret.labelSelector),
		zap.String("field-selector", ret.fieldSelector),
	)
	rs := &storage_v1.StorageClassList{ListMeta: meta_v1.ListMeta{Continue: ""}}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		rs, err = c.StorageV1().StorageClasses().List(ctx, meta_v1.ListOptions{Limit: batchLimit, Continue: rs.Continue})
		cancel()
		if err != nil {
			if retryLeft > 0 &&
				!IsRetryableAPIError(err) &&
				(strings.Contains(err.Error(), "too old to display a consistent") ||
					strings.Contains(err.Error(), "inconsistent")) {
				// e.g. The provided continue parameter is too old to display a consistent list result. You can start a new list without the continue parameter, or use the continue token in this response to retrieve the remainder of the results. Continuing with the provided token results in an inconsistent list - objects that were created, modified, or deleted between the time the first chunk was returned and now may show up in the list.
				lg.Warn("stale list response, retrying for consistent list", zap.Error(err))
				time.Sleep(15 * time.Second)
				return listStorageClass(lg, c, batchLimit, batchInterval, retryLeft-1, opts...)
			}
			return nil, err
		}
		storageclass = append(storageclass, rs.Items...)
		remained := int64Value(rs.RemainingItemCount)
		lg.Info("storageclass",
			zap.Int64("remained", remained),
			zap.Int("items", len(rs.Items)),
		)
		if rs.Continue == "" {
			break
		}
		time.Sleep(batchInterval)
	}
	lg.Info("listed storageclass", zap.Int("storageclass", len(storageclass)))
	return storageclass, err
}
