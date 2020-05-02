// Package hollownodes implements Hollow Nodes.
// ref. https://github.com/kubernetes/kubernetes/blob/master/pkg/kubemark/hollow_kubelet.go
//
// The purpose is to make it easy to run on EKS.
// ref. https://github.com/kubernetes/kubernetes/blob/master/test/kubemark/start-kubemark.sh
//
package hollownodes

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/eksconfig"
	k8s_client "github.com/aws/aws-k8s-tester/pkg/k8s-client"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/exec"
)

// Config defines hollow nodes configuration.
type Config struct {
	Logger *zap.Logger
	Stopc  chan struct{}

	EKSConfig *eksconfig.Config
	K8SClient k8s_client.EKS

	ECRAPI ecriface.ECRAPI
}

// Tester defines hollow nodes tester.
type Tester interface {
	// Create installs hollow nodes.
	Create() error
	// Delete deletes hollow nodes.
	Delete() error
}

func NewTester(cfg Config) (Tester, error) {
	ts := &tester{cfg: cfg}
	if ts.cfg.EKSConfig.AddOnHollowNodes.Remote {
		var err error
		ts.ng, err = CreateNodeGroup(NodeGroupConfig{
			Logger:         ts.cfg.Logger,
			Client:         ts.cfg.K8SClient,
			Stopc:          ts.cfg.Stopc,
			Nodes:          ts.cfg.EKSConfig.AddOnHollowNodes.Nodes,
			NodeLabels:     ts.cfg.EKSConfig.AddOnHollowNodes.NodeLabels,
			MaxOpenFiles:   ts.cfg.EKSConfig.AddOnHollowNodes.MaxOpenFiles,
			KubectlPath:    ts.cfg.EKSConfig.KubectlPath,
			KubeConfigPath: ts.cfg.EKSConfig.KubeConfigPath,
		})
		if err != nil {
			return nil, err
		}
	}
	return ts, nil
}

type tester struct {
	cfg Config
	ng  NodeGroup
}

func (ts *tester) Create() (err error) {
	if ts.cfg.EKSConfig.AddOnHollowNodes.Created {
		ts.cfg.Logger.Info("skipping create AddOnHollowNodes")
		return nil
	}

	ts.cfg.Logger.Info("starting hollow nodes testing")
	ts.cfg.EKSConfig.AddOnHollowNodes.Created = true
	ts.cfg.EKSConfig.Sync()
	createStart := time.Now()

	defer func() {
		ts.cfg.EKSConfig.AddOnHollowNodes.CreateTook = time.Since(createStart)
		ts.cfg.EKSConfig.AddOnHollowNodes.CreateTookString = ts.cfg.EKSConfig.AddOnHollowNodes.CreateTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	if err := k8s_client.CreateNamespace(ts.cfg.Logger, ts.cfg.K8SClient.KubernetesClientSet(), ts.cfg.EKSConfig.AddOnHollowNodes.Namespace); err != nil {
		return err
	}

	switch {
	case ts.cfg.EKSConfig.AddOnHollowNodes.Remote == false:
		if err = ts.ng.Start(); err != nil {
			return err
		}
		time.Sleep(10 * time.Second)
		_, ts.cfg.EKSConfig.AddOnHollowNodes.CreatedNodeNames, err = ts.ng.CheckNodes()
		if err != nil {
			return err
		}

	case ts.cfg.EKSConfig.AddOnHollowNodes.Remote == true:
		if err = ts.checkECR(); err != nil {
			return err
		}
		if err = ts.createConfigMap(); err != nil {
			return err
		}
		if err = ts.createDeployment(); err != nil {
			return err
		}
		if err = ts.waitDeployment(); err != nil {
			return err
		}
		if err = ts.checkNodes(); err != nil {
			return err
		}
	}

	waitDur, retryStart := 5*time.Minute, time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			ts.cfg.Logger.Warn("health check aborted")
			return nil
		case <-time.After(5 * time.Second):
		}
		err = ts.cfg.K8SClient.CheckHealth()
		if err == nil {
			break
		}
		ts.cfg.Logger.Warn("health check failed", zap.Error(err))
	}
	if err == nil {
		ts.cfg.Logger.Info("health check success after load testing")
	} else {
		ts.cfg.Logger.Warn("health check failed after load testing", zap.Error(err))
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) Delete() (err error) {
	if !ts.cfg.EKSConfig.AddOnHollowNodes.Created {
		ts.cfg.Logger.Info("skipping delete AddOnHollowNodes")
		return nil
	}

	deleteStart := time.Now()
	defer func() {
		ts.cfg.EKSConfig.AddOnHollowNodes.DeleteTook = time.Since(deleteStart)
		ts.cfg.EKSConfig.AddOnHollowNodes.DeleteTookString = ts.cfg.EKSConfig.AddOnHollowNodes.DeleteTook.String()
		ts.cfg.EKSConfig.Sync()
	}()

	var errs []string

	switch {
	case ts.cfg.EKSConfig.AddOnHollowNodes.Remote == true:
		ts.ng.Stop()
		if len(ts.cfg.EKSConfig.AddOnHollowNodes.CreatedNodeNames) == 0 {
			_, ts.cfg.EKSConfig.AddOnHollowNodes.CreatedNodeNames, err = ts.ng.CheckNodes()
			if err != nil {
				errs = append(errs, err.Error())
			}
		}

	case ts.cfg.EKSConfig.AddOnHollowNodes.Remote == false:
		panic("not implemented")
	}

	if err := ts.deleteCreatedNodes(); err != nil {
		errs = append(errs, err.Error())
	}

	switch {
	case ts.cfg.EKSConfig.AddOnHollowNodes.Remote == false:

	case ts.cfg.EKSConfig.AddOnHollowNodes.Remote == true:
		if err := ts.deleteDeployment(); err != nil {
			errs = append(errs, err.Error())
		}
		if err := ts.deleteConfigMap(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if err := k8s_client.DeleteNamespaceAndWait(ts.cfg.Logger,
		ts.cfg.K8SClient.KubernetesClientSet(),
		ts.cfg.EKSConfig.AddOnHollowNodes.Namespace,
		k8s_client.DefaultNamespaceDeletionInterval,
		k8s_client.DefaultNamespaceDeletionTimeout); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete hollow nodes namespace (%v)", err))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	ts.cfg.EKSConfig.AddOnHollowNodes.Created = false
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkECR() error {
	ts.cfg.Logger.Info("describing ECR repositories")
	out, err := ts.cfg.ECRAPI.DescribeRepositories(&ecr.DescribeRepositoriesInput{
		RepositoryNames: aws.StringSlice([]string{ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryName}),
	})
	if err != nil {
		return err
	}
	if len(out.Repositories) != 1 {
		return fmt.Errorf("%q expected 1 ECR repository, got %d", ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryName, len(out.Repositories))
	}
	repo := out.Repositories[0]
	arn := aws.StringValue(repo.RepositoryArn)
	name := aws.StringValue(repo.RepositoryName)
	uri := aws.StringValue(repo.RepositoryUri)
	ts.cfg.Logger.Info(
		"described ECR repository",
		zap.String("arn", arn),
		zap.String("name", name),
		zap.String("uri", uri),
	)

	if name != ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryName {
		return fmt.Errorf("unexpected ECR repository name %q", name)
	}
	if uri != ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryURI {
		return fmt.Errorf("unexpected ECR repository uri %q", uri)
	}

	ts.cfg.Logger.Info("describing images")
	imgOut, err := ts.cfg.ECRAPI.DescribeImages(&ecr.DescribeImagesInput{
		RepositoryName: aws.String(ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryName),
		ImageIds: []*ecr.ImageIdentifier{
			{
				ImageTag: aws.String(ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryImageTag),
			},
		},
	})
	if err != nil {
		return err
	}
	if len(imgOut.ImageDetails) == 0 {
		return fmt.Errorf("image tag %q not found", ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryImageTag)
	}
	ts.cfg.Logger.Info("described images", zap.Int("images", len(imgOut.ImageDetails)))
	for i, img := range imgOut.ImageDetails {
		ts.cfg.Logger.Info("found an image",
			zap.Int("index", i),
			zap.String("requested-tag", ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryImageTag),
			zap.Strings("returned-tags", aws.StringValueSlice(img.ImageTags)),
			zap.String("digest", aws.StringValue(img.ImageDigest)),
			zap.String("pushed-at", humanize.Time(aws.TimeValue(img.ImagePushedAt))),
			zap.String("size", humanize.Bytes(uint64(aws.Int64Value(img.ImageSizeInBytes)))),
		)
	}
	return nil
}

const (
	hollowNodesKubeConfigConfigMapName     = "hollow-nodes-kubeconfig-config-map"
	hollowNodesKubeConfigConfigMapFileName = "hollow-nodes-kubeconfig-config-map.yaml"
)

func (ts *tester) createConfigMap() error {
	ts.cfg.Logger.Info("creating config map")

	b, err := ioutil.ReadFile(ts.cfg.EKSConfig.KubeConfigPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnHollowNodes.Namespace).
		Create(
			ctx,
			&v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "ConfigMap",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      hollowNodesKubeConfigConfigMapName,
					Namespace: ts.cfg.EKSConfig.AddOnHollowNodes.Namespace,
					Labels: map[string]string{
						"name": hollowNodesKubeConfigConfigMapName,
					},
				},
				Data: map[string]string{
					hollowNodesKubeConfigConfigMapFileName: string(b),
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return err
	}

	ts.cfg.Logger.Info("created config map")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteConfigMap() error {
	ts.cfg.Logger.Info("deleting config map")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		CoreV1().
		ConfigMaps(ts.cfg.EKSConfig.AddOnHollowNodes.Namespace).
		Delete(
			ctx,
			hollowNodesKubeConfigConfigMapName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted config map")
	return ts.cfg.EKSConfig.Sync()
}

const (
	hollowNodesDeploymentName = "hollow-nodes-deployment"
	hollowNodesAppName        = "hollow-nodes-app"
)

// TODO: use "ReplicationController"?

// TODO: not working for now
// kubelet_node_status.go:92] Unable to register node "fake-node-000000-8pkvl" with API server: nodes "fake-node-000000-8pkvl" is forbidden: node "ip-192-168-83-61.us-west-2.compute.internal" is not allowed to modify node "fake-node-000000-8pkvl"
// need remove "NodeRestriction" from "kube-apiserver --enable-admission-plugins"
// ref. https://github.com/kubernetes/kubernetes/issues/47695
// ref. https://kubernetes.io/docs/reference/access-authn-authz/node

func (ts *tester) createDeployment() error {
	ngType := "custom"
	if ts.cfg.EKSConfig.IsEnabledAddOnManagedNodeGroups() {
		ngType = "managed"
	}

	testerCmd := fmt.Sprintf("/aws-k8s-tester eks create hollow-nodes --kubectl=/kubectl --kubeconfig=%s --prefix=%s --nodes=%d --clients=%d --client-qps=%f --client-burst=%d",
		"/opt/"+hollowNodesKubeConfigConfigMapFileName,
		ts.cfg.EKSConfig.AddOnHollowNodes.NodeLabelPrefix,
		ts.cfg.EKSConfig.AddOnHollowNodes.Nodes,
		ts.cfg.EKSConfig.Clients,
		ts.cfg.EKSConfig.ClientQPS,
		ts.cfg.EKSConfig.ClientBurst,
	)

	image := ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryURI + ":" + ts.cfg.EKSConfig.AddOnHollowNodes.RepositoryImageTag

	ts.cfg.Logger.Info("creating hollow nodes Deployment", zap.String("image", image), zap.String("tester-command", testerCmd))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnHollowNodes.Namespace).
		Create(
			ctx,
			&appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      hollowNodesDeploymentName,
					Namespace: ts.cfg.EKSConfig.AddOnHollowNodes.Namespace,
					Labels: map[string]string{
						"app": hollowNodesAppName,
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: aws.Int32(ts.cfg.EKSConfig.AddOnHollowNodes.DeploymentReplicas),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": hollowNodesAppName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": hollowNodesAppName,
							},
						},
						Spec: v1.PodSpec{
							// TODO: set resource limits
							Containers: []v1.Container{
								{
									Name:            hollowNodesAppName,
									Image:           image,
									ImagePullPolicy: v1.PullAlways,

									Command: []string{
										"/bin/sh",
										"-ec",
										testerCmd,
									},

									// grant access "/dev/kmsg"
									SecurityContext: &v1.SecurityContext{
										Privileged: aws.Bool(true),
									},

									// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
									VolumeMounts: []v1.VolumeMount{
										{ // to execute
											Name:      hollowNodesKubeConfigConfigMapName,
											MountPath: "/opt",
										},
										{ // for hollow node kubelet, kubelet requires "/dev/kmsg"
											Name:      "kmsg",
											MountPath: "/dev/kmsg",
										},
										{ // to write
											Name:      "var-log",
											MountPath: "/var/log",
										},
									},
								},
							},

							// ref. https://kubernetes.io/docs/concepts/cluster-administration/logging/
							Volumes: []v1.Volume{
								{ // to execute
									Name: hollowNodesKubeConfigConfigMapName,
									VolumeSource: v1.VolumeSource{
										ConfigMap: &v1.ConfigMapVolumeSource{
											LocalObjectReference: v1.LocalObjectReference{
												Name: hollowNodesKubeConfigConfigMapName,
											},
											DefaultMode: aws.Int32(0777),
										},
									},
								},
								{ // for hollow node kubelet
									Name: "kmsg",
									VolumeSource: v1.VolumeSource{
										HostPath: &v1.HostPathVolumeSource{
											Path: "/dev/kmsg",
										},
									},
								},
								{ // to write
									Name: "var-log",
									VolumeSource: v1.VolumeSource{
										EmptyDir: &v1.EmptyDirVolumeSource{},
									},
								},
							},

							NodeSelector: map[string]string{
								// do not deploy in fake nodes, obviously
								"NGType": ngType,
							},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create hollow node Deployment (%v)", err)
	}
	return nil
}

func (ts *tester) deleteDeployment() error {
	ts.cfg.Logger.Info("deleting deployment")
	foreground := metav1.DeletePropagationForeground
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.K8SClient.KubernetesClientSet().
		AppsV1().
		Deployments(ts.cfg.EKSConfig.AddOnHollowNodes.Namespace).
		Delete(
			ctx,
			hollowNodesDeploymentName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
	cancel()
	if err != nil {
		return err
	}
	ts.cfg.Logger.Info("deleted deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) waitDeployment() error {
	ts.cfg.Logger.Info("waiting for hollow nodes Deployment")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	output, err := exec.New().CommandContext(
		ctx,
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig="+ts.cfg.EKSConfig.KubeConfigPath,
		"--namespace="+ts.cfg.EKSConfig.AddOnHollowNodes.Namespace,
		"describe",
		"deployment",
		hollowNodesDeploymentName,
	).CombinedOutput()
	cancel()
	if err != nil {
		return fmt.Errorf("'kubectl describe deployment' failed %v", err)
	}
	out := string(output)
	fmt.Printf("\n\n\"kubectl describe deployment\" output:\n%s\n\n", out)

	ready := false
	waitDur := 5*time.Minute + time.Duration(ts.cfg.EKSConfig.AddOnHollowNodes.DeploymentReplicas)*time.Minute
	retryStart := time.Now()
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("check aborted")
		case <-time.After(15 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		dresp, err := ts.cfg.K8SClient.KubernetesClientSet().
			AppsV1().
			Deployments(ts.cfg.EKSConfig.AddOnHollowNodes.Namespace).
			Get(ctx, hollowNodesDeploymentName, metav1.GetOptions{})
		cancel()
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
		if available && dresp.Status.AvailableReplicas >= ts.cfg.EKSConfig.AddOnHollowNodes.DeploymentReplicas {
			ready = true
			break
		}
	}
	if !ready {
		// TODO: return error...
		// return errors.New("Deployment not ready")
		ts.cfg.Logger.Warn("Deployment not ready")
	}

	ts.cfg.Logger.Info("waited for hollow nodes Deployment")
	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) checkNodes() error {
	argsGetCSRs := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"get",
		"csr",
		"-o=wide",
	}
	cmdGetCSRs := strings.Join(argsGetCSRs, " ")

	argsGetNodes := []string{
		ts.cfg.EKSConfig.KubectlPath,
		"--kubeconfig=" + ts.cfg.EKSConfig.KubeConfigPath,
		"get",
		"nodes",
		"--show-labels",
		"-o=wide",
	}
	cmdGetNodes := strings.Join(argsGetNodes, " ")

	expectedNodes := ts.cfg.EKSConfig.AddOnHollowNodes.Nodes * int(ts.cfg.EKSConfig.AddOnHollowNodes.DeploymentReplicas)
	retryStart, waitDur := time.Now(), 5*time.Minute+2*time.Second*time.Duration(expectedNodes)
	ts.cfg.Logger.Info("checking nodes readiness", zap.Duration("wait", waitDur), zap.Int("expected-nodes", expectedNodes))
	ready := false
	for time.Now().Sub(retryStart) < waitDur {
		select {
		case <-ts.cfg.Stopc:
			return errors.New("checking node aborted")
		case <-time.After(5 * time.Second):
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		nodes, err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("get nodes failed", zap.Error(err))
			continue
		}
		items := nodes.Items

		createdNodeNames := make([]string, 0)
		readies := 0
		for _, node := range items {
			labels := node.GetLabels()
			if !strings.HasPrefix(labels["NGName"], ts.cfg.EKSConfig.AddOnHollowNodes.NodeLabelPrefix) {
				continue
			}
			nodeName := node.GetName()

			ts.cfg.Logger.Info("checking node readiness", zap.String("name", nodeName))
			for _, cond := range node.Status.Conditions {
				if cond.Status != v1.ConditionTrue {
					continue
				}
				if cond.Type != v1.NodeReady {
					continue
				}
				ts.cfg.Logger.Info("checked node readiness",
					zap.String("name", nodeName),
					zap.String("type", fmt.Sprintf("%s", cond.Type)),
					zap.String("status", fmt.Sprintf("%s", cond.Status)),
				)
				createdNodeNames = append(createdNodeNames, nodeName)
				readies++
				break
			}
		}
		ts.cfg.Logger.Info("nodes",
			zap.Int("current-ready-nodes", readies),
			zap.Int("desired-ready-nodes", expectedNodes),
		)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err := exec.New().CommandContext(ctx, argsGetCSRs[0], argsGetCSRs[1:]...).CombinedOutput()
		cancel()
		out := string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl get csr' failed", zap.Error(err))
		}
		fmt.Printf("\n\n\"%s\":\n%s\n", cmdGetCSRs, out)

		ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
		output, err = exec.New().CommandContext(ctx, argsGetNodes[0], argsGetNodes[1:]...).CombinedOutput()
		cancel()
		out = string(output)
		if err != nil {
			ts.cfg.Logger.Warn("'kubectl get nodes' failed", zap.Error(err))
		}
		fmt.Printf("\n\"%s\":\n%s\n", cmdGetNodes, out)

		ts.cfg.EKSConfig.AddOnHollowNodes.CreatedNodeNames = createdNodeNames
		ts.cfg.EKSConfig.Sync()
		if readies >= expectedNodes {
			ready = true
			break
		}
	}
	if !ready {
		return fmt.Errorf("NG %q not ready", ts.cfg.EKSConfig.AddOnHollowNodes.NodeLabelPrefix)
	}

	return ts.cfg.EKSConfig.Sync()
}

func (ts *tester) deleteCreatedNodes() error {
	var errs []string

	ts.cfg.Logger.Info("deleting node objects", zap.Int("created-nodes", len(ts.cfg.EKSConfig.AddOnHollowNodes.CreatedNodeNames)))
	deleted := 0
	foreground := metav1.DeletePropagationForeground
	for i, nodeName := range ts.cfg.EKSConfig.AddOnHollowNodes.CreatedNodeNames {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := ts.cfg.K8SClient.KubernetesClientSet().CoreV1().Nodes().Delete(
			ctx,
			nodeName,
			metav1.DeleteOptions{
				GracePeriodSeconds: aws.Int64(0),
				PropagationPolicy:  &foreground,
			},
		)
		cancel()
		if err != nil {
			ts.cfg.Logger.Warn("failed to delete node", zap.Int("index", i), zap.String("name", nodeName), zap.Error(err))
			errs = append(errs, err.Error())
		} else {
			ts.cfg.Logger.Info("deleted node", zap.Int("index", i), zap.String("name", nodeName))
			deleted++
		}
	}
	ts.cfg.Logger.Info("deleted node objects", zap.Int("deleted", deleted), zap.Int("created-nodes", len(ts.cfg.EKSConfig.AddOnHollowNodes.CreatedNodeNames)))

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}
