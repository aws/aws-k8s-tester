//go:build e2e

package fips

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-k8s-tester/internal/awssdk"
	fwext "github.com/aws/aws-k8s-tester/internal/e2e"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
	testenv     env.Environment
	ecrEndpoint string
	isFIPSEndpoint      bool
	accountID   string
	ecrImage    *string
	region      string

	// Rendered manifests selected based on region
	renderedRegistryFIPS    []byte
	renderedRegistryNonFIPS []byte
)

// fipsRegions is the set of AWS regions that support FIPS ECR endpoints.
var fipsRegions = map[string]bool{
	"us-east-1":     true,
	"us-east-2":     true,
	"us-west-1":     true,
	"us-west-2":     true,
	"us-gov-east-1": true,
	"us-gov-west-1": true,
}

type manifestVars struct {
	ECREndpoint string
}

func resolveECREndpoint(accountID, region string) (endpoint string, fips bool) {
	if fipsRegions[region] {
		return fmt.Sprintf("%s.dkr.ecr-fips.%s.amazonaws.com", accountID, region), true
	}
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region), false
}

func createECRSecret(ctx context.Context, config *envconf.Config) (context.Context, error) {
	if !isFIPSEndpoint {
		log.Println("Non-FIPS region, skipping ECR secret creation")
		return ctx, nil
	}

	clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return ctx, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Delete existing secret if present (token may be expired)
	_ = clientset.CoreV1().Secrets("default").Delete(ctx, "ecr-creds", metav1.DeleteOptions{})

	log.Println("Creating ECR secret for FIPS endpoint authentication")
	password := os.Getenv("ECR_PASSWORD")
	auth := base64.StdEncoding.EncodeToString([]byte("AWS:" + password))
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ecr-creds",
			Namespace: "default",
		},
		Type: v1.SecretTypeDockerConfigJson,
		StringData: map[string]string{
			".dockerconfigjson": fmt.Sprintf(
				`{"auths":{"%s":{"username":"AWS","password":"%s","auth":"%s"}}}`,
				ecrEndpoint, password, auth,
			),
		},
	}
	_, err = clientset.CoreV1().Secrets("default").Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return ctx, fmt.Errorf("failed to create ECR secret: %w", err)
	}
	log.Println("ECR secret created successfully")
	return ctx, nil
}

func deleteECRSecret(ctx context.Context, config *envconf.Config) (context.Context, error) {
	if !isFIPSEndpoint {
		return ctx, nil
	}
	clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
	if err != nil {
		return ctx, err
	}
	err = clientset.CoreV1().Secrets("default").Delete(ctx, "ecr-creds", metav1.DeleteOptions{})
	if err != nil {
		log.Printf("Warning: failed to delete ECR secret: %v", err)
	}
	return ctx, nil
}

func waitForSeed(ctx context.Context, clientset *kubernetes.Clientset, dsName string) error {
	log.Printf("Waiting for %s seed container to complete...", dsName)
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		pods, err := clientset.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: "name=" + dsName,
		})
		if err != nil {
			return err
		}
		for _, pod := range pods.Items {
			req := clientset.CoreV1().Pods("default").GetLogs(pod.Name, &v1.PodLogOptions{
				Container: "seed-image",
			})
			stream, err := req.Stream(ctx)
			if err != nil {
				break
			}
			body, _ := io.ReadAll(stream)
			stream.Close()
			logs := string(body)
			if strings.Contains(logs, "level=fatal") {
				return fmt.Errorf("%s seed failed: %s", dsName, logs)
			}
			if strings.Contains(logs, "Writing manifest to image destination") {
				log.Printf("%s seed completed successfully", dsName)
				return nil
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("%s seed did not complete within timeout", dsName)
}

func TestMain(m *testing.M) {
	accountID = os.Getenv("AWS_ACCOUNT_ID")
	if accountID == "" {
		log.Fatal("AWS_ACCOUNT_ID is not set.")
	}
	ecrImage = flag.String("ecrImage", "my-images:latest", "ECR image name:tag to use for seeding")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		log.Fatalf("failed to initialize test environment: %v", err)
	}
	testenv = env.NewWithConfig(cfg)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	testenv = testenv.WithContext(ctx)

	// Resolve region and ECR endpoint
	awsCfg := awssdk.NewConfig()
	region = awsCfg.Region
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		log.Fatal("AWS region is not set. Set AWS_REGION or configure your AWS SDK.")
	}
	ecrEndpoint, isFIPSEndpoint = resolveECREndpoint(accountID, region)
	imageRef := ecrEndpoint + "/" + *ecrImage
	log.Printf("[INFO] Region: %s, FIPS: %v, ECR Endpoint: %s", region, isFIPSEndpoint, ecrEndpoint)

	// Select and render manifests based on region
	var regFIPSRaw, regNonFIPSRaw []byte
	if isFIPSEndpoint {
		regFIPSRaw = registryFIPSManifest
		regNonFIPSRaw = registryNonFIPSManifest
	} else {
		regFIPSRaw = registryFIPSNonFIPSRegionManifest
		regNonFIPSRaw = registryNonFIPSNonFIPSRegionManifest
	}

	vars := manifestVars{ECREndpoint: imageRef}
	renderedRegistryFIPS, err = fwext.RenderManifests(regFIPSRaw, vars)
	if err != nil {
		log.Fatalf("failed to render registry-fips manifest: %v", err)
	}
	renderedRegistryNonFIPS, err = fwext.RenderManifests(regNonFIPSRaw, vars)
	if err != nil {
		log.Fatalf("failed to render registry-nonfips manifest: %v", err)
	}

	testenv.Setup(
		createECRSecret,
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			clientset, err := kubernetes.NewForConfig(config.Client().RESTConfig())
			if err != nil {
				return ctx, fmt.Errorf("failed to create Kubernetes client: %w", err)
			}
			if err := fwext.ApplyManifests(config.Client().RESTConfig(), renderedRegistryFIPS); err != nil {
				return ctx, fmt.Errorf("failed to apply registry-fips manifest: %w", err)
			}
			log.Println("registry-fips DaemonSet deployed")

			if err := fwext.ApplyManifests(config.Client().RESTConfig(), renderedRegistryNonFIPS); err != nil {
				return ctx, fmt.Errorf("failed to apply registry-nonfips manifest: %w", err)
			}
			log.Println("registry-nonfips DaemonSet deployed")

			// Wait for both DaemonSets to be ready before tests create pods
			for _, name := range []string{"registry-fips", "registry-nonfips"} {
				ds := appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				}
				log.Printf("Waiting for %s DaemonSet to be ready...", name)
				err := wait.For(
					fwext.NewConditionExtension(config.Client().Resources()).DaemonSetReady(&ds),
					wait.WithContext(ctx),
				)
				if err != nil {
					return ctx, fmt.Errorf("%s DaemonSet not ready: %w", name, err)
				}
				log.Printf("%s DaemonSet is ready", name)
			}

			// Verify seed containers successfully copied images
			for _, dsName := range []string{"registry-fips", "registry-nonfips"} {
				if err := waitForSeed(ctx, clientset, dsName); err != nil {
					return ctx, fmt.Errorf("seed verification failed for %s: %w", dsName, err)
				}
			}

			return ctx, nil
		},
	)

	testenv.Finish(
		func(ctx context.Context, config *envconf.Config) (context.Context, error) {
			fwext.DeleteManifests(config.Client().RESTConfig(), renderedRegistryFIPS)
			fwext.DeleteManifests(config.Client().RESTConfig(), renderedRegistryNonFIPS)
			fwext.DeleteManifests(config.Client().RESTConfig(), testPodsManifest)
			return ctx, nil
		},
		deleteECRSecret,
	)

	os.Exit(testenv.Run(m))
}
