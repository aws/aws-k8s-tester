package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/e2e/framework/utils"
	"github.com/onsi/ginkgo"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultNamespaceDeletionTimeout = 10 * time.Minute
)

type NamespaceManager struct {
	cs kubernetes.Interface

	namespacesToDelete []string
}

func NewNamespaceManager(cs kubernetes.Interface) *NamespaceManager {
	return &NamespaceManager{
		cs: cs,
	}
}

func (m *NamespaceManager) Cleanup(ctx context.Context) error {
	var errMsgs []string
	for _, ns := range m.namespacesToDelete {
		ctx, cancel := context.WithTimeout(ctx, DefaultNamespaceDeletionTimeout)
		if err := m.DeleteNamespace(ctx, ns); err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("Couldn't delete ns: %q: %s (%#v)", ns, err, err))
		}
		cancel()
	}
	if len(errMsgs) != 0 {
		return errors.New(strings.Join(errMsgs, ","))
	}
	return nil
}

func (m *NamespaceManager) CreateNamespaceUnique(ctx context.Context, baseName string) (*corev1.Namespace, error) {
	name, err := m.findAvailableNamespaceName(ctx, baseName)
	if err != nil {
		return nil, err
	}
	return m.CreateNamespace(ctx, name)
}

func (m *NamespaceManager) CreateNamespace(ctx context.Context, name string) (*corev1.Namespace, error) {
	namespaceObj := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "",
		},
		Status: corev1.NamespaceStatus{},
	}

	m.namespacesToDelete = append(m.namespacesToDelete, name)

	var namespace *corev1.Namespace
	ginkgo.By(fmt.Sprintf("Creating namespace %q for this suite.", name))
	if err := wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		namespace, err = m.cs.CoreV1().Namespaces().Create(ctx, namespaceObj, metav1.CreateOptions{})
		cancel()
		if err != nil {
			utils.Logf("Unexpected error while creating namespace: %v", err)
			return false, nil
		}
		return true, nil
	}, ctx.Done()); err != nil {
		return nil, err
	}
	return namespace, nil
}

// DeleteNamespace deletes the provided namespace, waits for it to be completely deleted.
func (m *NamespaceManager) DeleteNamespace(ctx context.Context, namespace string) error {
	startTime := time.Now()

	ginkgo.By(fmt.Sprintf("Deleting namespace %q for this suite.", namespace))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := m.cs.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	cancel()
	if err != nil {
		if apierrs.IsNotFound(err) {
			utils.Logf("Namespace %v was already deleted", namespace)
			return nil
		}
		return err
	}

	// wait for namespace to delete or timeout.
	if err := wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err := m.cs.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
		cancel()
		if err != nil {
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			utils.Logf("Error while waiting for namespace to be terminated: %v", err)
			return false, nil
		}
		return false, nil
	}, ctx.Done()); err != nil {
		return err
	}

	utils.Logf("namespace %v deletion completed in %s", namespace, time.Since(startTime))
	return nil
}

// findAvailableNamespaceName random namespace name starting with baseName.
func (m *NamespaceManager) findAvailableNamespaceName(ctx context.Context, baseName string) (string, error) {
	var name string
	err := wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		name = fmt.Sprintf("%v-%v", baseName, utils.RandomSuffix())
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err := m.cs.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		cancel()
		if err == nil {
			// Already taken
			return false, nil
		}
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		utils.Logf("Unexpected error while getting namespace: %v", err)
		return false, nil
	}, ctx.Done())

	return name, err
}
