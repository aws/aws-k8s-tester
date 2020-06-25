package resource

import (
	"context"
	"time"

	"github.com/aws/aws-k8s-tester/e2e/framework/utils"
	log "github.com/cihub/seelog"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type ServiceManager struct {
	cs kubernetes.Interface
}

func NewServiceManager(cs kubernetes.Interface) *ServiceManager {
	return &ServiceManager{
		cs: cs,
	}
}

// WaitServiceHasEndpointsNum waits until the service has the expected number of endpoints
func (m *ServiceManager) WaitServiceHasEndpointsNum(ctx context.Context, svc *corev1.Service, epCounts int) (*corev1.Service, error) {
	if err := wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ep, err := m.cs.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		cancel()
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		observedEpCount := 0
		for _, sub := range ep.Subsets {
			observedEpCount += len(sub.Addresses)
		}
		if observedEpCount == epCounts {
			return true, nil
		}
		return false, nil
	}, ctx.Done()); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	svc, err := m.cs.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
	cancel()
	return svc, err
}

// WaitServiceHasEndpointIP waits for the service to have a specific endpoint IP
// TODO deal with port
func (m *ServiceManager) WaitServiceHasEndpointIP(ctx context.Context, svc *corev1.Service, ip string) (*corev1.Service, error) {
	if err := wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		ep, err := m.cs.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		cancel()
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		for _, sub := range ep.Subsets {
			for _, subAddr := range sub.Addresses {
				log.Debugf("endpoints have %s want %s", subAddr.IP, ip)
				if subAddr.IP == ip {
					log.Debugf("endpoint found")
					return true, nil
				}
			}
		}
		return false, nil
	}, ctx.Done()); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	svc, err := m.cs.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
	cancel()
	return svc, err
}
