package eks

import (
	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ts *Tester) createNamespace() error {
	ts.lg.Info("creating namespace", zap.String("namespace", ts.cfg.Name))
	_, err := ts.k8sClientSet.
		CoreV1().
		Namespaces().
		Create(&v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ts.cfg.Name,
				Labels: map[string]string{
					"name": ts.cfg.Name,
				},
			},
		})
	if err != nil {
		return err
	}
	ts.lg.Info("created namespace", zap.String("namespace", ts.cfg.Name))
	return ts.cfg.Sync()
}

func (ts *Tester) deleteNamespace() error {
	if ts.k8sClientSet == nil {
		ts.lg.Warn("skipping namespace delete; empty k8s client")
		return nil
	}

	ts.lg.Info("deleting namespace", zap.String("namespace", ts.cfg.Name))
	foreground := metav1.DeletePropagationForeground
	err := ts.k8sClientSet.
		CoreV1().
		Namespaces().
		Delete(
			ts.cfg.Name,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	if err != nil {
		return err
	}
	ts.lg.Info("deleted namespace", zap.String("namespace", ts.cfg.Name))
	return ts.cfg.Sync()
}
