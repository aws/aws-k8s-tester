// Package nlb implements NLB plugin.
package nlb

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/pkg/aws/elb"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/utils/exec"
)

// Config defines ALB configuration.
type Config struct {
	Logger    *zap.Logger
	Stopc     chan struct{}
	Sig       chan os.Signal
	EKSConfig *eksconfig.Config
	K8SClient k8sClientSetGetter
	ELB2API   elbv2iface.ELBV2API
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
	if ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created {
		ts.cfg.Logger.Info("skipping create AddOnNLBHelloWorld")
		return nil
	}

	ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created = true
	ts.cfg.EKSConfig.Sync()

	createStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.CreateTookString = ts.cfg.EKSConfig.AddOnNLBHelloWorld.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := ts.createNamespace(); err != nil {
		return err
	}
	if err := ts.createDeployment(); err != nil {
		return err
	}
	if err := ts.waitDeployment(); err != nil {
		return err
	}
	if err := ts.createService(); err != nil {
		return err
	}
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() error {
	if !ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created {
		ts.cfg.Logger.Info("skipping delete AddOnNLBHelloWorld")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeleteTookString = ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string
	if err := ts.deleteService(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world Service (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Service")
	time.Sleep(time.Minute)

	if err := ts.deleteDeployment(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world Deployment (%v)", err))
	}
	ts.cfg.Logger.Info("wait for a minute after deleting Deployment")
	time.Sleep(time.Minute)

	/*
	   # NLB tags
	   kubernetes.io/service-name
	   leegyuho-test-prod-nlb-hello-world/hello-world-service

	   kubernetes.io/cluster/leegyuho-test-prod
	   owned
	*/
	if err := elb.DeleteELBv2(
		ts.cfg.Logger,
		ts.cfg.ELB2API,
		ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBARN,
		ts.cfg.EKSConfig.Parameters.VPCID,
		map[string]string{
			"kubernetes.io/cluster/" + ts.cfg.EKSConfig.Name: "owned",
			"kubernetes.io/service-name":                     ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace + "/" + nlbHelloWorldServiceName,
		},
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB hello-world (%v)", err))
	}

	if err := ts.deleteNamespace(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete NLB namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnNLBHelloWorld.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createNamespace() error {
	ts.cfg.Logger.Info("creating namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace))
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Create(&v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
				Labels: map[string]string{
					"name": ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
				},
			},
		})
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("created namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteNamespace() error {
	ts.cfg.Logger.Info("deleting namespace", zap.String("namespace", ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace))
	foreground := metav1.DeletePropagationForeground
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Namespaces().
		Delete(
			ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
			&metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	if err != nil {
		// ref. https://github.com/aws/aws-k8s-tester/issues/79
		if !strings.Contains(err.Error(), ` not found`) {
			return err
		}
	}
	ts.cfg.Logger.Info("deleted namespace", zap.Error(err))
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createDeployment() error {
	ts.cfg.Logger.Info("creating NLB hello-world Deployment")
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
		Create(&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      nlbHelloWorldDeploymentName,
				Namespace: ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
				Labels: map[string]string{
					"app": nlbHelloWorldAppName,
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentReplicas),
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
		Deployments(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
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

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for NLB hello-world Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
		"describe",
		"deployment",
		nlbHelloWorldDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 5*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentReplicas)*time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-ts.cfg.Sig:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
			Get(nlbHelloWorldDeploymentName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get Deployment (%v)", err)
		}
		ts.cfg.Logger.Info("get deployment",
			zap.Int32("desired-replicas", dresp.Status.Replicas),
			zap.Int32("available-replicas", dresp.Status.AvailableReplicas),
			zap.Int32("unavailable-replicas", dresp.Status.UnavailableReplicas),
			zap.Int32("ready-replicas", dresp.Status.ReadyReplicas),
		)
		available := false
		for _, cond := range dresp.Status.Conditions {
			ts.cfg.Logger.Info("condition",
				zap.String("last-updated", cond.LastUpdateTime.String()),
				zap.String("type", string(cond.Type)),
				zap.String("status", string(cond.Status)),
				zap.String("reason", cond.Reason),
				zap.String("message", cond.Message),
			)
			if cond.Status != v1.ConditionTrue {
				continue
			}
			if cond.Type == appsv1.DeploymentAvailable {
				available = true
				break
			}
		}
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnNLBHelloWorld.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		return errors.New("Deployment not ready")
	}

	ts.cfg.Logger.Info("waited for NLB hello-world Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) createService() error {
	ts.cfg.Logger.Info("creating NLB hello-world Service")
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		Services(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
		Create(&v1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      nlbHelloWorldServiceName,
				Namespace: ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
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

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		clusterInfoOut, err := exec.New().CommandContext(
			ctx,
			ts.cfg.EKSConfig.KubectlPath,
			"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
			"--namespace="+ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace,
			"describe",
			"svc",
			nlbHelloWorldServiceName,
		).CombinedOutput()
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl describe svc' failed", zap.Error(err))
		} else {
			out := string(clusterInfoOut)
			fmt.Printf("\n\n\"kubectl describe svc %s\" output:\n%s\n\n", nlbHelloWorldServiceName, out)
		}

		ts.cfg.Logger.Info("querying NLB hello-world Service for HTTP endpoint")
		so, err := ts.cfg.K8SClient.KubernetesClientSet().
			CoreV1().
			Services(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
			Get(nlbHelloWorldServiceName, metav1.GetOptions{})
		if err != nil {
			ts.cfg.Logger.Error("failed to get NLB hello-world Service; retrying", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		ts.cfg.Logger.Info(
			"NLB hello-world Service has been linked to LoadBalancer",
			zap.String("load-balancer", fmt.Sprintf("%+v", so.Status.LoadBalancer)),
		)
		for _, ing := range so.Status.LoadBalancer.Ingress {
			ts.cfg.Logger.Info(
				"NLB hello-world Service has been linked to LoadBalancer.Ingress",
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

	// TODO: is there any better way to find out the NLB name?
	ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBName = strings.Split(hostName, "-")[0]
	ss := strings.Split(hostName, ".")[0]
	ss = strings.Replace(ss, "-", "/", -1)
	ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBARN = fmt.Sprintf(
		"arn:aws:elasticloadbalancing:%s:%s:loadbalancer/net/%s",
		ts.cfg.EKSConfig.Region,
		ts.cfg.EKSConfig.Status.AWSAccountID,
		ss,
	)
	ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL = "http://" + hostName
	ts.cfg.EKSConfig.Sync()

	fmt.Printf("\nNLB hello-world ARN %s\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBARN)
	fmt.Printf("NLB hello-world Name %s\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.NLBName)
	fmt.Printf("NLB hello-world URL %s\n\n", ts.cfg.EKSConfig.AddOnNLBHelloWorld.URL)

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
		Services(ts.cfg.EKSConfig.AddOnNLBHelloWorld.Namespace).
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
