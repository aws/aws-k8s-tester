package client

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_client "k8s.io/client-go/kubernetes"
)

func ListPods(
	lg *zap.Logger,
	c k8s_client.Interface,
	namespace string,
	batchLimit int64,
	batchInterval time.Duration,
	opts ...OpOption) (pods []core_v1.Pod, err error) {
	ns, err := listPods(lg, c, namespace, batchLimit, batchInterval, 5, opts...)
	return ns, err
}

func listPods(
	lg *zap.Logger,
	c k8s_client.Interface,
	namespace string,
	batchLimit int64,
	batchInterval time.Duration,
	retryLeft int,
	opts ...OpOption) (pods []core_v1.Pod, err error) {
	ret := Op{}
	ret.applyOpts(opts)

	lg.Info("listing pods",
		zap.String("namespace", namespace),
		zap.Int64("batch-limit", batchLimit),
		zap.Duration("batch-interval", batchInterval),
		zap.String("label-selector", ret.labelSelector),
		zap.String("field-selector", ret.fieldSelector),
	)
	rs := &core_v1.PodList{ListMeta: meta_v1.ListMeta{Continue: ""}}
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		rs, err = c.CoreV1().Pods(namespace).List(ctx, meta_v1.ListOptions{Limit: batchLimit, Continue: rs.Continue})
		cancel()
		if err != nil {
			if retryLeft > 0 &&
				!IsRetryableAPIError(err) &&
				(strings.Contains(err.Error(), "too old to display a consistent") ||
					strings.Contains(err.Error(), "inconsistent")) {
				// e.g. The provided continue parameter is too old to display a consistent list result. You can start a new list without the continue parameter, or use the continue token in this response to retrieve the remainder of the results. Continuing with the provided token results in an inconsistent list - objects that were created, modified, or deleted between the time the first chunk was returned and now may show up in the list.
				lg.Warn("stale list response, retrying for consistent list", zap.Error(err))
				time.Sleep(15 * time.Second)
				return listPods(lg, c, namespace, batchLimit, batchInterval, retryLeft-1, opts...)
			}
			return nil, err
		}
		pods = append(pods, rs.Items...)
		remained := int64Value(rs.RemainingItemCount)
		lg.Info("pods",
			zap.Int64("remained", remained),
			zap.Int("items", len(rs.Items)),
		)
		if rs.Continue == "" {
			break
		}
		time.Sleep(batchInterval)
	}
	lg.Info("listed pods", zap.Int("pods", len(pods)))
	return pods, err
}
