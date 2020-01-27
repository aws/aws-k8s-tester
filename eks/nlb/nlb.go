// Package nlb implements NLB plugin.
package nlb

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
)

// Config defines ALB configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
	ELB2API   elbv2iface.ELBV2API
	Namespace string
}

type k8sClientSetGetter interface {
	KubernetesClientSet() *clientset.Clientset
}

// Tester defines Job tester.
type Tester interface {
	// Create creates Job objects, and waits for completion.
	Create() error
	// Delete deletes all Job objects.
	Delete() error
}

// New creates a new Job tester.
func New(cfg Config) (Tester, error) {
	return &tester{cfg: cfg}, nil
}

type tester struct {
	cfg Config
}

const (
	nlbHelloWorldAppName        = "hello-world"
	nlbHelloWorldAppImageName   = "dockercloud/hello-world"
	nlbHelloWorldDeploymentName = "hello-world-deployment"
	nlbHelloWorldServiceName    = "hello-world-service"
)

func (ts *tester) Create() error {
	if err := ts.createDeployment(); err != nil {
		return err
	}
	if err := ts.createService(); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	var errs []string
	if err := ts.deleteService(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world Service (%v)", err))
	}
	time.Sleep(20 * time.Second)

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world Deployment (%v)", err))
	}
	time.Sleep(30 * time.Second)

	// TODO: clean up target groups, LBs
	if 1 == 2 {
		ts.cfg.ELB2API.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{})
		ts.cfg.ELB2API.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{})
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeployment() error {
	ts.cfg.Logger.Info("creating NLB hello-world Deployment")
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.Namespace).
		Create(&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      nlbHelloWorldDeploymentName,
				Namespace: ts.cfg.Namespace,
				Labels: map[string]string{
					"app": nlbHelloWorldAppName,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: aws.Int32(3),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": nlbHelloWorldAppName,
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": nlbHelloWorldAppName,
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:            nlbHelloWorldAppName,
								Image:           nlbHelloWorldAppImageName,
								ImagePullPolicy: v1.PullAlways,
								Ports: []v1.ContainerPort{
									{
										Protocol:      v1.ProtocolTCP,
										ContainerPort: 80,
									},
								},
							},
						},
					},
				},
			},
		})
	if err != nil {
		return fmt.Errorf("failed to create NLB hello-world Deployment (%v)", err)
	}
	ts.cfg.Logger.Info("created NLB hello-world Deployment")

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting NLB hello-world Deployment")
	foreground := metav1.DeletePropagationForeground
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.Namespace).
		Delete(
			nlbHelloWorldDeploymentName,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	if err != nil && !strings.Contains(err.Error(), " not found") {
		return fmt.Errorf("failed to delete NLB hello-world Deployment (%v)", err)
	}
	ts.cfg.Logger.Info("deleted NLB hello-world Deployment")

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createService() error {
	ts.cfg.Logger.Info("creating NLB hello-world Service")
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.Namespace).
		Create(&v1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      nlbHelloWorldServiceName,
				Namespace: ts.cfg.Namespace,
				Annotations: map[string]string{
					"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
				},
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"app": nlbHelloWorldAppName,
				},
				Type: v1.ServiceTypeLoadBalancer,
				Ports: []v1.ServicePort{
					{
						Protocol:   v1.ProtocolTCP,
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				},
			},
		})
	if err != nil {
		return fmt.Errorf("failed to create NLB hello-world Service (%v)", err)
	}
	ts.cfg.Logger.Info("created NLB hello-world Service")

	waitDur := 3 * time.Minute
	ts.cfg.Logger.Info("waiting for NLB hello-world Service", zap.Duration("wait", waitDur))
	select {
	case <-ts.cfg.Stopc:
		return errors.New("NLB hello-world Service creation aborted")
	case sig := <-ts.cfg.Sig:
		return fmt.Errorf("received os signal %v", sig)
	case <-time.After(waitDur):
	}

	hostName := ""
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("NLB hello-world Service creation aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("received os signal %v", sig)
		case <-time.After(5 * time.Second):
		}
		ts.cfg.Logger.Info("querying NLB hello-world Service for HTTP endpoint")
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.Namespace).
			Get(nlbHelloWorldServiceName, metav1.GetOptions{})
		if err != nil {
			ts.cfg.Logger.Error("failed to get NLB hello-world Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}
		ts.cfg.Logger.Info(
			"NLB hello-world Service returns LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"NLB hello-world Service returns LoadBalancer.Ingress",
				zap.String("ingress", fmt.Sprintf("%+v", ing)),
			)
			hostName = ing.Hostname
			break
		}
		if hostName != "" {
			ts.cfg.Logger.Info("found NLB host name", zap.String("host-name", hostName))
			break
		}
	}
	if hostName == "" {
		return errors.New("failed to find NLB host name")
	}

	ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL = "http://" + hostName
	println()
	fmt.Println("NLB hello-world URL:", ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL)
	println()
	ts.cfg.EKSConfig.Sync()

	ts.cfg.Logger.Info("waiting before testing hello-world Service")
	time.Sleep(20 * time.Second)

	retryStart = time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("hello-world Service creation aborted")
		case sig := <-ts.cfg.Sig:
			return fmt.Errorf("received os signal %v", sig)
		case <-time.After(5 * time.Second):
		}

		buf := bytes.NewBuffer(nil)
		err = httpReadInsecure(ts.cfg.Logger, ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL, buf)
		if err != nil {
			ts.cfg.Logger.Error("failed to read NLB hello-world Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		httpOutput := buf.String()
		fmt.Printf("\nNLB hello-world Service output:\n%s\n", httpOutput)

		if strings.Contains(httpOutput, `<h1>Hello world!</h1>`) {
			ts.cfg.Logger.Info(
				"read hello-world Service; exiting",
				zap.String("host-name", hostName),
			)
			break
		}

		ts.cfg.Logger.Warn("unexpected hello-world Service output; retrying")
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteService() error {
	ts.cfg.Logger.Info("deleting NLB hello-world Service")
	foreground := metav1.DeletePropagationForeground
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.Namespace).
		Delete(
			nlbHelloWorldServiceName,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	if err != nil && !strings.Contains(err.Error(), " not found") {
		return fmt.Errorf("failed to delete NLB hello-world Service (%v)", err)
	}
	ts.cfg.Logger.Info("deleted NLB hello-world Service", zap.Error(err))

	return ts.cfg.EKSConfig.Sync()
}

// curl -k [URL]
func httpReadInsecure(lg *zap.Logger, u string, wr io.Writer) error {
	lg.Info("reading", zap.String("url", u))
	cli := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}}
	r, err := cli.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		return fmt.Errorf("%q returned %d", u, r.StatusCode)
	}

	_, err = io.Copy(wr, r.Body)
	if err != nil {
		lg.Warn("failed to read", zap.String("url", u), zap.Error(err))
	} else {
		lg.Info("read", zap.String("url", u))
	}
	return err
}
