// Package csi_ebs tests the CSI drivers storage capabilities
package csi_ebs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/client"
	k8s_tester "github.com/aws/aws-k8s-tester/k8s-tester/tester"
	"github.com/aws/aws-k8s-tester/utils/rand"
	utils_time "github.com/aws/aws-k8s-tester/utils/time"
	"github.com/manifoldco/promptui"
	"go.uber.org/zap"

	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	storage_v1 "k8s.io/api/storage/v1"

	storagev1 "k8s.io/api/storage/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	api_resource "k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	Enable bool `json:"enable"`
	Prompt bool `json:"-"`

	Logger    *zap.Logger   `json:"-"`
	LogWriter io.Writer     `json:"-"`
	Stopc     chan struct{} `json:"-"`
	Client    client.Client `json:"-"`

	// MinimumNodes is the minimum number of Kubernetes nodes required for installing this addon.
	MinimumNodes int `json:"minimum-nodes"`
	// Namespace to create test resources.
	Namespace string `json:"namespace"`
}

const (
	chartName           string = "aws-ebs-csi-driver"
	chartRepoURL        string = "https://kubernetes-sigs.github.io/aws-ebs-csi-driver"
	storageClassName    string = "ebs-sc"
	pvcProvisionName    string = "ebs-provision-pvc"
	pvcSnapshotName     string = "ebs-snapshot-pvc"
	provisioner         string = "ebs.csi.aws.com"
	VolumeBindingMode   string = "WaitForFirstConsumer"
	provisionPodName    string = "provisionpod"
	snapshotPodName     string = "snapshotpod"
	provisionVolumeName string = "provisionvolume"
	snapshotVolumeName  string = "snapshotvolume"
	snapshotName        string = "testsnapshot"
	datapath            string = "/mnt/test/data"
	volMode             string = "v1.PersistentVolumeBlock"
	DefaultMinimumNodes int    = 1
)

var values = map[string]interface{}{
	"enableVolumeScheduling": true,
	"enableVolumeResizing":   true,
	"enableVolumeSnapshot":   true,
}

var foreground = meta_v1.DeletePropagationForeground

var graceperiod = int64(0)

var pkgName = path.Base(reflect.TypeOf(tester{}).PkgPath())

func NewDefault() *Config {
	return &Config{
		Enable:       false,
		Prompt:       true,
		MinimumNodes: DefaultMinimumNodes,
		Namespace:    pkgName + "-" + rand.String(10) + "-" + utils_time.GetTS(10),
	}
}

func New(cfg *Config) k8s_tester.Tester {
	return &tester{
		cfg: cfg,
	}
}

type tester struct {
	cfg *Config
}

func (ts *tester) Name() string { return pkgName }

func (ts *tester) Enabled() bool { return ts.cfg.Enable }

func (ts *tester) Apply() error {
	if ok := ts.runPrompt("apply"); !ok {
		return errors.New("cancelled")
	}
	if nodes, err := client.ListNodes(ts.cfg.Client); len(nodes) < ts.cfg.MinimumNodes || err != nil {
		return fmt.Errorf("failed to validate minimum nodes requirement %d (nodes %v, error %v)", ts.cfg.MinimumNodes, len(nodes), err)
	}
	if err := client.CreateNamespace(ts.cfg.Logger, ts.cfg.Client, ts.cfg.Namespace); err != nil {
		return err
	}
	if err := ts.installEBSHelmChart(); err != nil {
		return err
	}
	if err := ts.createEBSStorageClass(); err != nil {
		return err
	}
	if err := ts.createPersistentVolumeClaim(storageClassName); err != nil {
		return err
	}
	if err := ts.volumeProvisionTest(); err != nil {
		return err
	}
	if err := ts.volumeResizeTests(); err != nil {
		return err
	}
	return nil
}

func (ts *tester) Delete() error {
	if ok := ts.runPrompt("delete"); !ok {
		return errors.New("cancelled")
	}
	var errs []string
	if err := ts.deleteEBSHelmChart(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Helm Chart EBS (%v)", err))
	}
	if err := ts.deleteEBSStorageClass(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Helm Chart EBS (%v)", err))
	}
	if err := ts.deletePersistentVolumeClaim(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Helm Chart EBS (%v)", err))
	}
	if err := ts.deletevolumePods(); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete Helm Chart EBS (%v)", err))
	}
	if err := client.DeleteNamespaceAndWait(
		ts.cfg.Logger,
		ts.cfg.Client,
		ts.cfg.Namespace,
		client.DefaultNamespaceDeletionInterval,
		client.DefaultNamespaceDeletionTimeout,
		client.WithForceDelete(true),
	); err != nil {
		errs = append(errs, fmt.Sprintf("failed to delete namespace (%v)", err))
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}
	return nil
}

func (ts *tester) runPrompt(action string) (ok bool) {
	if ts.cfg.Prompt {
		msg := fmt.Sprintf("Ready to %q resources for the namespace %q, should we continue?", action, ts.cfg.Namespace)
		prompt := promptui.Select{
			Label: msg,
			Items: []string{
				"No, cancel it!",
				fmt.Sprintf("Yes, let's %q!", action),
			},
		}
		idx, answer, err := prompt.Run()
		if err != nil {
			panic(err)
		}
		if idx != 1 {
			fmt.Printf("cancelled %q [index %d, answer %q]\n", action, idx, answer)
			return false
		}
	}
	return true
}

var dirOrCreate = v1.HostPathDirectoryOrCreate

func (ts *tester) installEBSHelmChart() error {
	ts.cfg.Logger.Info("creating %s: %s", zap.String("Helm Chart", chartName))
	err := client.InstallHelm(client.HelmInstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Stopc:          ts.cfg.Stopc,
		Timeout:        5 * time.Minute,
		KubeConfigPath: ts.cfg.Client.Config().KubeconfigPath,
		Namespace:      "kube-system",
		ChartRepoURL:   chartRepoURL,
		ChartName:      chartName,
		ReleaseName:    chartName,
		Values:         values,
		LogFunc: func(format string, v ...interface{}) {
			ts.cfg.Logger.Info(fmt.Sprintf("[install] "+format, v...))
		},
	})
	if err != nil {
		if k8s_errors.IsAlreadyExists(err) {
			ts.cfg.Logger.Info("resource already exists", zap.String("Helm Chart", chartName))
			return nil
		}
		return fmt.Errorf("failed to create %s: %s (%v)", "Helm Chart", chartName, err)
	}
	ts.cfg.Logger.Info("Create resource", zap.String("Helm Chart", chartName))
	return nil
}

func (ts *tester) deleteEBSHelmChart() error {
	ts.cfg.Logger.Info("deleting %s: %s", zap.String("Helm Chart", chartName))
	err := client.UninstallHelm(client.HelmInstallConfig{
		Logger:         ts.cfg.Logger,
		LogWriter:      ts.cfg.LogWriter,
		Timeout:        3 * time.Minute,
		KubeConfigPath: ts.cfg.Client.Config().KubeconfigPath,
		Namespace:      "kube-system",
		ChartName:      chartName,
		ReleaseName:    chartName,
	})
	if err == nil {
		ts.cfg.Logger.Info("deleted Helm Chart", zap.String("namespace", ts.cfg.Namespace), zap.String("name", chartName))
		return nil
	}
	if k8s_errors.IsNotFound(err) || k8s_errors.IsGone(err) {
		ts.cfg.Logger.Info("Helm Chart already deleted", zap.String("namespace", ts.cfg.Namespace), zap.String("name", chartName), zap.Error(err))
		return nil
	}
	ts.cfg.Logger.Warn("failed to delete Helm Chart", zap.String("namespace", ts.cfg.Namespace), zap.String("name", chartName), zap.Error(err))
	return err
	// requires "k8s_errors.IsNotFound"
	// ref. https://github.com/aws/aws-k8s-tester/issues/79
}

func (ts *tester) createEBSStorageClass() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	firstConsumerBinding := storagev1.VolumeBindingWaitForFirstConsumer
	allowVolumeExpansion := true
	_, err = ts.cfg.Client.StorageV1().StorageClasses().Create(
		ctx,
		&storage_v1.StorageClass{
			TypeMeta: meta_v1.TypeMeta{
				APIVersion: "storage.k8s.io/v1",
				Kind:       "StorageClass",
			},
			ObjectMeta: meta_v1.ObjectMeta{
				Name: storageClassName,
			},
			Provisioner:          provisioner,
			AllowVolumeExpansion: &allowVolumeExpansion,
			VolumeBindingMode:    &firstConsumerBinding,
			Parameters: map[string]string{
				"type": "gp2",
			},
		},
		meta_v1.CreateOptions{},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create StorageClasses (%v)", err)
	}
	ts.cfg.Logger.Info("created a StorageClasses for EBS")
	return nil
}

func (ts *tester) deleteEBSStorageClass() (err error) {
	ts.cfg.Logger.Info("deleting storageClass for EBS")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	err = ts.cfg.Client.StorageV1().StorageClasses().Delete(
		ctx,
		storageClassName,
		meta_v1.DeleteOptions{
			PropagationPolicy: &foreground,
		},
	)
	cancel()
	if err != nil && !k8s_errors.IsNotFound(err) && !strings.Contains(err.Error(), "not found") {
		ts.cfg.Logger.Warn("failed to delete", zap.Error(err))
		return err
	}
	ts.cfg.Logger.Info("delete StorageClasses for EBS")
	return nil
}

func (ts *tester) createPersistentVolumeClaim(storageClass string) error {
	ts.cfg.Logger.Info("creating PersistentVolumeClaim for EBS, Provisioning test")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err := ts.cfg.Client.CoreV1().PersistentVolumeClaims(ts.cfg.Namespace).Create(
		ctx,
		&core_v1.PersistentVolumeClaim{
			TypeMeta: meta_v1.TypeMeta{
				APIVersion: "v1",
				Kind:       "PersistentVolumeClaim",
			},
			ObjectMeta: meta_v1.ObjectMeta{
				Name: pvcProvisionName,
			},
			Spec: core_v1.PersistentVolumeClaimSpec{
				AccessModes:      []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				StorageClassName: &storageClass,
				Resources: core_v1.ResourceRequirements{
					Requests: core_v1.ResourceList{
						core_v1.ResourceStorage: api_resource.MustParse("4Gi"),
					},
				},
			},
		},
		meta_v1.CreateOptions{},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolumeClaims (%v)", err)
	}
	ts.cfg.Logger.Info("created a PersistentVolumeClaims for EBS")
	return nil
}

func (ts *tester) deletePersistentVolumeClaim() error {
	ts.cfg.Logger.Info("deleting PersistentVolumeClaim for EBS, Provisioning test")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	err := ts.cfg.Client.CoreV1().PersistentVolumeClaims(ts.cfg.Namespace).Delete(
		ctx,
		pvcProvisionName,
		meta_v1.DeleteOptions{
			PropagationPolicy: &foreground,
		},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to delete PersistentVolumeClaim (%v)", err)
	}
	ts.cfg.Logger.Info("Deleted a PersistentVolumeClaim for EBS")
	return nil
}

//It should dynamically provision a volume from the PVC without pod mount/startup failure
func (ts *tester) volumeProvisionTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	var gracePeriod int64 = 1
	ts.cfg.Logger.Info("Creating Pod to test Volume Provisioning", zap.String("Pod", provisionPodName))
	_, err := ts.cfg.Client.
		CoreV1().Pods(ts.cfg.Namespace).Create(
		ctx,
		&v1.Pod{
			TypeMeta: meta_v1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: meta_v1.ObjectMeta{
				Name: provisionPodName,
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:       provisionPodName,
						Image:      "public.ecr.aws/hudsonbay/busybox:latest",
						WorkingDir: "/opt",
						// An imperative and easily debuggable container which reads/writes vol contents for
						// us to scan in the tests or by eye.
						// We expect that /opt is empty in the minimal containers which we use in this test.
						Command: []string{"/bin/sh", "-c", "while true ; do sleep 2; done "},
						VolumeMounts: []v1.VolumeMount{
							{
								Name:      provisionVolumeName,
								MountPath: "/opt/1",
							},
						},
					},
				},
				TerminationGracePeriodSeconds: &gracePeriod,
				Volumes: []v1.Volume{
					{
						Name: provisionVolumeName,
						VolumeSource: v1.VolumeSource{
							PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvcProvisionName,
							},
						},
					},
				},
			},
		},
		meta_v1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create VolumeProvision Pod for volumeProvisionTest test: (%v)", err)
	}
	//Wait for Pod to spawn
	time.Sleep(20 * time.Second)
	ts.cfg.Logger.Info("Retrieving Dynamic Provisioed Claim on Pod", zap.String("Claim", pvcProvisionName))
	claim, err := ts.cfg.Client.CoreV1().PersistentVolumeClaims(ts.cfg.Namespace).Get(
		ctx,
		pvcProvisionName,
		meta_v1.GetOptions{},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to GET PersistentVolumeClaims pvcProvisionName (%v)", err)
	}
	pv, err := GetBoundPV(ts, claim)
	ts.cfg.Logger.Info("Got PV from Pod", zap.String("PV", pv.ObjectMeta.Name))
	if err != nil {
		return fmt.Errorf("failed to GetBoundPV (%v)", err)
	}
	expectedCapacity := resource.MustParse("4Gi")
	pvCapacity := pv.Spec.Capacity[v1.ResourceName(v1.ResourceStorage)]
	ts.cfg.Logger.Info("Checking Desired Capacity vs Actual PV Capacity")
	if expectedCapacity.Value() == pvCapacity.Value() {
		ts.cfg.Logger.Info("[PASSED] expectedCapacity did equal volume Capacity", zap.String(expectedCapacity.String(), pvCapacity.String()))
	} else {
		return fmt.Errorf("expectedCapacity did not equal volume Capacity (%v)", err)
	}
	return nil
}

//It should handle resizing on running, and stopped pods
func (ts *tester) volumeResizeTests() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	//Resize testing
	ts.cfg.Logger.Info("Starting PVC Resizing Tests")
	pvc, err := ts.cfg.Client.CoreV1().PersistentVolumeClaims(ts.cfg.Namespace).Get(
		ctx,
		pvcProvisionName,
		meta_v1.GetOptions{},
	)
	ts.cfg.Logger.Info("Found PVC for Resizing Tests", zap.String("PVC", pvcProvisionName))
	if err != nil {
		return fmt.Errorf("failed to GET PersistentVolumeClaims pvcProvisionName (%v)", err)
	}
	//Make Deepcopy of PVC with new size, and apply to current PVC
	ts.cfg.Logger.Info("Chaning PVC Size of running pod from 4GI to", zap.String("Size", "6Gi"))
	newSize := resource.MustParse("6Gi")
	newPVC, err := ExpandPVCSize(ts, pvc, newSize)
	if newPVC == nil {
		return fmt.Errorf("failed to create Resize of PVC (%v)", err)
	}
	//Check if PVC is being updated
	pvcSize := newPVC.Spec.Resources.Requests[v1.ResourceStorage]
	if pvcSize.Cmp(newSize) != 0 {
		return fmt.Errorf("error updating pvc size %v", err)
	}
	cancel()
	//Wait for PVC to come back healthy
	ts.cfg.Logger.Info("Waiting on PVC ReSize for max timeout of 8 minutes...")
	err = WaitForControllerVolumeResize(ts, newPVC, 8*time.Minute)
	if err != nil {
		return fmt.Errorf("VolumeResize resize timeout occured due to error (%v)", err)
	}
	ts.cfg.Logger.Info("[PASSED] PVC ReSize on running Pod", zap.String("New Size", "6Gi"))
	return nil
}

//Cleanup Testing Pods
func (ts *tester) deletevolumePods() error {
	ts.cfg.Logger.Info("deleting Pods for EBS CSI tests")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := ts.cfg.Client.CoreV1().Pods(ts.cfg.Namespace).Delete(
		ctx,
		provisionPodName,
		meta_v1.DeleteOptions{
			GracePeriodSeconds: &graceperiod,
			PropagationPolicy:  &foreground,
		},
	)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to delete Pod (%v)", err)
	}
	ts.cfg.Logger.Info("Deleted a Pod for EBS Tests")
	return nil
}
