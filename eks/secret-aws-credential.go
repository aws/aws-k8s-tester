package eks

import (
	"fmt"
	"io/ioutil"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (ts *Tester) createSecretAWSCredential() error {
	d, err := ioutil.ReadFile(ts.cfg.Status.AWSCredentialPath)
	if err != nil {
		return err
	}
	size := humanize.Bytes(uint64(len(d)))

	ts.lg.Info("creating and mounting AWS credential as Secret",
		zap.String("file-path", ts.cfg.Status.AWSCredentialPath),
		zap.String("size", size),
	)

	name := "aws-cred-aws-k8s-tester"

	/*
	  kubectl \
	    --namespace=kube-system \
	    create secret generic aws-cred-aws-k8s-tester \
	    --from-file=aws-cred-aws-k8s-tester/[FILE-PATH]
	*/
	so, err := ts.k8sClientSet.
		CoreV1().
		Secrets("kube-system").
		Create(&v1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "kube-system",
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{
				name: d,
			},
		})
	if err != nil {
		return fmt.Errorf("failed to create AWS credential as Secret (%v)", err)
	}

	/*
	  kubectl \
	    --namespace=kube-system \
	    get secret aws-cred-aws-k8s-tester \
	    --output=yaml
	*/
	ts.lg.Info("mounted AWS credential as Secret",
		zap.String("file-path", ts.cfg.Status.AWSCredentialPath),
		zap.String("size", size),
		zap.String("created-timestamp", so.GetCreationTimestamp().String()),
	)
	return ts.cfg.Sync()
}
