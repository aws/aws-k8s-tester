package eks

import (
	"context"
	"fmt"
	"github.com/aws/aws-k8s-tester/pkg/randutil"
	"github.com/dustin/go-humanize"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"os"
	"os/signal"
	"sigs.k8s.io/yaml"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var cfg = stresser2{}

func newCreateStresserV2() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stresser2",
		Short: "Creates stresser v2",
		Run:   createStresserFuncV2,
	}
	cmd.PersistentFlags().IntVar(&cfg.N, "number", 5, "number of go routines")
	cmd.PersistentFlags().DurationVar(&cfg.duration, "duration", 10*time.Minute, "duration of the simulation")
	cmd.PersistentFlags().IntVar(&cfg.objectSize, "object-size", 8, "object size, by default 8 bytes")
	cmd.PersistentFlags().IntVar(&cfg.secretNum, "secret-num", 10, "secret object in default namespace, no more than 500")
	cmd.PersistentFlags().StringVar(&cfg.busyboxImage, "busybox-image", "", "busy box ecr image uri")
	return cmd
}

type stresser2 struct {
	N int
	duration time.Duration
	objectSize int
	secretNum int
	busyboxImage string
}

func createStresserFuncV2(cmd *cobra.Command, args []string) {
	if cfg.secretNum > 500 {
		fmt.Fprintf(os.Stderr, "fail to start stresser v2, due to secret-num bigger than 500")
		os.Exit(1)
	}
	if cfg.busyboxImage == "" {
		fmt.Fprintf(os.Stderr, "fail empty busybox ecr Image")
		os.Exit(1)
	}

	// creates the in cluster cfg
	config, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		panic(err.Error())
	}

	// increase qps and burst to avoid throttling on client-side
	config.QPS = 100000000
	config.Burst = 200000000

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	terminateC := make(chan struct{}, 1)

	go func(){
		select {
		case sig := <- sigs:
			fmt.Println(fmt.Sprintf("received signal, %s", sig.String()))
			for i := 0; i < 2 * cfg.N + cfg.secretNum; i++ {
				terminateC <- struct{}{}
			}

		case <- time.After(cfg.duration):
		}
	}()

	var wg sync.WaitGroup
	wg.Add(2 * cfg.N + cfg.secretNum)

	for i := 0; i < cfg.secretNum; i++ {
		go startWriteSecrets(config, &wg, cfg.duration, cfg.objectSize, terminateC)
	}

	for i := 0; i < cfg.N; i++ {
		go startWriteConfigMaps(config, &wg, cfg.duration, cfg.objectSize, terminateC)
		go startWritePods(config, &wg, cfg.duration, cfg.busyboxImage, cfg.objectSize, terminateC)
	}

	wg.Wait()
	fmt.Println("finish all jobs")
}

func startWriteSecrets(config *restclient.Config, wg *sync.WaitGroup, duration time.Duration, objectSize int, terminateC <-chan struct{}) {
	defer wg.Done()

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// define the deployment client
	secretsClient := clientset.CoreV1().Secrets(corev1.NamespaceDefault)
	id := uuid.Must(uuid.NewRandom()).String()
	secretName := "demo-secret" + id
	val := randutil.String(objectSize)

	stopc := make(chan struct{})
	donecCloseOnce := new(sync.Once)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <- stopc:
				fmt.Println("received stop signal")
				return
			case <- ticker.C:
				func() {
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
					defer cancel()
					defer func() {
						time.Sleep(2*time.Second)
						fmt.Println("Deleting deployment...")
						err := secretsClient.Delete(context.TODO(), secretName, metav1.DeleteOptions{})
						if err != nil {
							fmt.Printf("fail delete secret %s due to %v.\n", secretName, err)
						} else {
							fmt.Printf("Deleted secret %s.\n", secretName)
						}
					}()

					// define the deployment object
					secret := &corev1.Secret{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind: "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							// create random number to distinguish between multiple deployment
							Name:      secretName,
							Namespace: corev1.NamespaceDefault,
							Labels: map[string]string{
								"name": secretName,
							},
						},
						Data: map[string][]byte{secretName: []byte(val)},
					}
					secretRawData, err := yaml.Marshal(secret)
					if err != nil {
						panic(fmt.Sprintf("fail marshal secret %v due to (%+v)", secret, err))
					}

					// Create Secret
					fmt.Println("Creating secret...")
					result, _ := secretsClient.Create(context.TODO(), secret, metav1.CreateOptions{})
					fmt.Printf("Created secret %q with size %s.\n", result.GetObjectMeta().GetName(), humanize.Bytes(uint64(len(secretRawData))))

					// forever loop to update deployment
					for {
						select {
						case <- ctx.Done():
							fmt.Println(fmt.Sprintf("time out stop updating the secret %s", secretName))
							cancel()
							return
						case <- stopc:
							fmt.Println("received stop signal")
							return
						default:
						}
						// Update Deployment
						fmt.Printf("Updating secret %q.\n", secretName)
						//    You have two options to Update() this Secret:
						//
						//    1. Modify the "deployment" variable and call: Update(secret).
						//       This works like the "kubectl replace" command and it overwrites/loses changes
						//       made by other clients between you Create() and Update() the object.
						//    2. Modify the "result" returned by Get() and retry Update(result) until
						//       you no longer get a conflict error. This way, you can preserve changes made
						//       by other clients between Create() and Update(). This is implemented below
						//			 using the retry utility package included with client-go. (RECOMMENDED)
						//
						// More Info:
						// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency

						retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							// Retrieve the latest version of Deployment before attempting update
							// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
							result, getErr := secretsClient.Get(context.TODO(), secretName, metav1.GetOptions{})
							if getErr != nil {
								fmt.Printf("The error message is: %q", getErr.Error())
							}

							// update annotation
							if result.Annotations == nil {
								newAnnotation := make(map[string]string)
								newAnnotation["KEY"] = "value"
								result.Annotations = newAnnotation
								fmt.Println("Add Annotations for secret")
							} else {
								result.Annotations = nil
								fmt.Println("Reset Annotations for secret")
							}

							_, updateErr := secretsClient.Update(context.TODO(), result, metav1.UpdateOptions{})
							return updateErr
						})
						if retryErr != nil {
							fmt.Printf("Update failed: %v", retryErr)
						}
						fmt.Println("Updated secret...")
					}
				}()
			}
		}
	}()

	select {
	case <- time.After(duration):
		fmt.Println(fmt.Sprintf("exit after timeout %v", duration))
		donecCloseOnce.Do(func() {
			close(stopc)
		})
		// enough time to gracefully shutdown and clean up spawned deployments
		time.Sleep(30 * time.Second)
		return
	case <- terminateC:
		fmt.Println("startWriteSecrets received signal from terminateC, stopping")
		donecCloseOnce.Do(func() {
			close(stopc)
		})
		// enough time to gracefully shutdown and clean up spawned deployments
		time.Sleep(30 * time.Second)
		return
	}
}

func startWriteConfigMaps(config *restclient.Config, wg *sync.WaitGroup, duration time.Duration, objectSize int, terminateC <-chan struct{}) {
	defer wg.Done()

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// define the configmap client
	cmClient := clientset.CoreV1().ConfigMaps(corev1.NamespaceDefault)
	id := uuid.Must(uuid.NewRandom()).String()
	cmName := "demo-configmap" + id
	val := randutil.String(objectSize)

	stopc := make(chan struct{})
	donecCloseOnce := new(sync.Once)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <- stopc:
				fmt.Println("received stop signal")
				return
			case <- ticker.C:
				func() {
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
					defer cancel()
					defer func() {
						time.Sleep(2*time.Second)
						fmt.Println("Deleting configmap...")
						err := cmClient.Delete(context.TODO(), cmName, metav1.DeleteOptions{})
						if err != nil {
							fmt.Printf("fail delete configmap %s due to %v.\n", cmName, err)
						} else {
							fmt.Printf("Deleted configmap %s.\n", cmName)
						}
					}()

					// define the deployment object
					cm := &corev1.ConfigMap{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind: "ConfigMap",
						},
						ObjectMeta: metav1.ObjectMeta{
							// create random number to distinguish between multiple deployment
							Name:      cmName,
							Namespace: corev1.NamespaceDefault,
							Labels: map[string]string{
								"name": cmName,
							},
						},
						Data: map[string]string{cmName: val},
					}
					configMapRawData, err := yaml.Marshal(cm)
					if err != nil {
						panic(fmt.Sprintf("fail marshal configmap %v due to (%+v)", cm, err))
					}

					// Create Secret
					fmt.Println("Creating configmap...")
					result, _ := cmClient.Create(context.TODO(), cm, metav1.CreateOptions{})
					fmt.Printf("Created configmap %q with size %s.\n", result.GetObjectMeta().GetName(), humanize.Bytes(uint64(len(configMapRawData))))

					// forever loop to update configmap
					for {
						select {
						case <- ctx.Done():
							fmt.Println(fmt.Sprintf("time out stop updating the secret %s", cmName))
							cancel()
							return
						case <- stopc:
							fmt.Println("received stop signal")
							return
						default:
						}
						// Update Deployment
						fmt.Printf("Updating configmap %q.\n", cmName)
						//    You have two options to Update() this Secret:
						//
						//    1. Modify the "deployment" variable and call: Update(configmap).
						//       This works like the "kubectl replace" command and it overwrites/loses changes
						//       made by other clients between you Create() and Update() the object.
						//    2. Modify the "result" returned by Get() and retry Update(result) until
						//       you no longer get a conflict error. This way, you can preserve changes made
						//       by other clients between Create() and Update(). This is implemented below
						//			 using the retry utility package included with client-go. (RECOMMENDED)
						//
						// More Info:
						// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency

						retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							// Retrieve the latest version of Deployment before attempting update
							// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
							result, getErr := cmClient.Get(context.TODO(), cmName, metav1.GetOptions{})
							if getErr != nil {
								fmt.Printf("The error message is: %q", getErr.Error())
							}

							// update annotation
							if result.Annotations == nil {
								newAnnotation := make(map[string]string)
								newAnnotation["KEY"] = "value"
								result.Annotations = newAnnotation
								fmt.Println("Add Annotations for secret")
							} else {
								result.Annotations = nil
								fmt.Println("Reset Annotations for secret")
							}

							_, updateErr := cmClient.Update(context.TODO(), result, metav1.UpdateOptions{})
							return updateErr
						})
						if retryErr != nil {
							fmt.Printf("Update failed: %v", retryErr)
						}
						fmt.Println("Updated configmap...")
					}
				}()
			}
		}
	}()

	select {
	case <- time.After(duration):
		fmt.Println(fmt.Sprintf("exit after timeout %v", duration))
		donecCloseOnce.Do(func() {
			close(stopc)
		})
		// enough time to gracefully shutdown and clean up spawned deployments
		time.Sleep(30 * time.Second)
		return
	case <- terminateC:
		fmt.Println("startWriteConfigMaps received signal from terminateC, stopping")
		donecCloseOnce.Do(func() {
			close(stopc)
		})
		// enough time to gracefully shutdown and clean up spawned config maps
		time.Sleep(30 * time.Second)
		return
	}
}

func startWritePods(config *restclient.Config, wg *sync.WaitGroup, duration time.Duration, busyboxImageURI string, objectSize int, terminateC <-chan struct{}) {
	defer wg.Done()

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	podNamespace := "pod-namespace"
	_, err = clientset.CoreV1().Namespaces().Create(context.TODO(),  &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podNamespace,
			Namespace: "",
		},
		Status: corev1.NamespaceStatus{},
	}, metav1.CreateOptions{})

	if err != nil && !strings.Contains(err.Error(), "already exists") {
		panic(fmt.Sprintf("fail create namespace %s due to (%+v)", podNamespace, err))
	}
	if err == nil {
		fmt.Println(fmt.Sprintf("succeed created namespace %s with status", podNamespace))
	}

	// define the configmap client
	podClient := clientset.CoreV1().Pods(podNamespace)
	id := uuid.Must(uuid.NewRandom()).String()
	podName := "demo-pod" + id
	val := randutil.String(objectSize)

	stopc := make(chan struct{})
	donecCloseOnce := new(sync.Once)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <- stopc:
				fmt.Println("received stop signal")
				return
			case <- ticker.C:
				func() {
					ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
					defer cancel()
					defer func() {
						time.Sleep(2*time.Second)
						fmt.Println("Deleting pod...")
						err := podClient.Delete(context.TODO(), podName, metav1.DeleteOptions{})
						if err != nil {
							fmt.Printf("fail delete pod %s due to %v.\n", podName, err)
						} else {
							fmt.Printf("Deleted pod %s.\n", podName)
						}
					}()

					// define the deployment object
					po := &corev1.Pod{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind: "Pod",
						},
						ObjectMeta: metav1.ObjectMeta{
							// create random number to distinguish between multiple deployment
							Name:      podName,
							Namespace: podNamespace,
							Labels: map[string]string{
								"name": podName,
							},
						},
						Spec: corev1.PodSpec{
							// spec.template.spec.restartPolicy: Unsupported value: "Always": supported values: "OnFailure", "Never"
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:            podName,
									Image:           busyboxImageURI,
									ImagePullPolicy: corev1.PullAlways,
									Command: []string{
										"/bin/sh",
										"-ec",
										fmt.Sprintf("echo -n '%s' >> /config/output.txt", val),
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "config",
											MountPath: "/config",
										},
									},
								},
							},

							Volumes: []corev1.Volume{
								{
									Name: "config",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{},
									},
								},
							},
						},
					}
					poRawData, err := yaml.Marshal(po)
					if err != nil {
						panic(fmt.Sprintf("fail marshal pod %v due to (%+v)", po, err))
					}

					// Create Secret
					fmt.Println("Creating pod...")
					result, _ := podClient.Create(context.TODO(), po, metav1.CreateOptions{})
					fmt.Printf("Created pod %q with size %s.\n", result.GetObjectMeta().GetName(), humanize.Bytes(uint64(len(poRawData))))

					// forever loop to update configmap
					for {
						select {
						case <- ctx.Done():
							fmt.Println(fmt.Sprintf("time out stop updating the pod %s", podName))
							cancel()
							return
						case <- stopc:
							fmt.Println("received stop signal")
							return
						default:
						}
						// Update Deployment
						fmt.Printf("Updating pod %q.\n", podName)
						//    You have two options to Update() this Pod:
						//
						//    1. Modify the "deployment" variable and call: Update(configmap).
						//       This works like the "kubectl replace" command and it overwrites/loses changes
						//       made by other clients between you Create() and Update() the object.
						//    2. Modify the "result" returned by Get() and retry Update(result) until
						//       you no longer get a conflict error. This way, you can preserve changes made
						//       by other clients between Create() and Update(). This is implemented below
						//			 using the retry utility package included with client-go. (RECOMMENDED)
						//
						// More Info:
						// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency

						retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
							// Retrieve the latest version of Deployment before attempting update
							// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
							result, getErr := podClient.Get(context.TODO(), podName, metav1.GetOptions{})
							if getErr != nil {
								fmt.Printf("The error message is: %q", getErr.Error())
							}

							// update annotation
							if result.Annotations == nil {
								newAnnotation := make(map[string]string)
								newAnnotation["KEY"] = "value"
								result.Annotations = newAnnotation
								fmt.Println("Add Annotations for secret")
							} else {
								result.Annotations = nil
								fmt.Println("Reset Annotations for secret")
							}

							_, updateErr := podClient.Update(context.TODO(), result, metav1.UpdateOptions{})
							return updateErr
						})
						if retryErr != nil {
							fmt.Printf("Update failed: %v", retryErr)
						}
						fmt.Println("Updated pod...")
					}
				}()
			}
		}
	}()

	select {
	case <- time.After(duration):
		fmt.Println(fmt.Sprintf("exit after timeout %v", duration))
		donecCloseOnce.Do(func() {
			close(stopc)
		})
		// enough time to gracefully shutdown and clean up spawned deployments
		time.Sleep(30 * time.Second)

		err = clientset.CoreV1().Namespaces().Delete(context.TODO(), podNamespace, metav1.DeleteOptions{})
		if err != nil {
			fmt.Println(fmt.Sprintf("fail delete namespace %s due to (%+v)", podNamespace, err))
		}
		return

	case <- terminateC:
		fmt.Println("startWritePods received signal from terminateC, stopping")
		donecCloseOnce.Do(func() {
			close(stopc)
		})
		// enough time to gracefully shutdown and clean up spawned deployments
		time.Sleep(30 * time.Second)

		err = clientset.CoreV1().Namespaces().Delete(context.TODO(), podNamespace, metav1.DeleteOptions{})
		if err != nil {
			fmt.Println(fmt.Sprintf("fail delete namespace %s", podNamespace))
		}
		return
	}
}
