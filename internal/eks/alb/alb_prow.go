package alb

import (
	"github.com/aws/aws-sdk-go/aws"
	gyaml "github.com/ghodss/yaml"
	"k8s.io/api/core/v1"
	"k8s.io/test-infra/prow/config"
)

// ConfigProwJobYAML defines ALB Prow job configuration.
type ConfigProwJobYAML struct {
	AWSTESTER_EKS_KUBETEST_EMBEDDED_BINARY string

	AWSTESTER_EKS_WAIT_BEFORE_DOWN string
	AWSTESTER_EKS_DOWN             string

	AWSTESTER_EKS_ENABLE_WORKER_NODE_HA       string
	AWSTESTER_EKS_ENABLE_NODE_SSH string

	AWSTESTER_EKS_AWSTESTER_IMAGE string

	AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE string
	AWSTESTER_EKS_WORKER_NODE_ASG_MIN       string
	AWSTESTER_EKS_WORKER_NODE_ASG_MAX       string

	AWSTESTER_EKS_LOG_DEBUG               string
	AWSTESTER_EKS_LOG_ACCESS              string
	AWSTESTER_EKS_UPLOAD_AWS_TESTER_LOGS  string
	AWSTESTER_EKS_ALB_UPLOAD_TESTER_LOGS  string
	AWSTESTER_EKS_UPLOAD_WORKER_NODE_LOGS string

	AWSTESTER_EKS_ALB_ENABLE string

	AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE string

	AWSTESTER_EKS_ALB_TARGET_TYPE string
	AWSTESTER_EKS_ALB_TEST_MODE   string

	AWSTESTER_EKS_ALB_TEST_SCALABILITY            string
	AWSTESTER_EKS_ALB_TEST_METRICS                string
	AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS        string
	AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES          string
	AWSTESTER_EKS_ALB_TEST_CLIENTS                string
	AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS        string
	AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE          string
	AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD string
	AWSTESTER_EKS_ALB_TEST_EXPECT_QPS             string
}

// CreateProwJobYAML returns Prow job configurations.
func CreateProwJobYAML(cfg ConfigProwJobYAML) (string, error) {
	jcfg := config.JobConfig{
		Presets: []config.Preset{
			{
				Labels: map[string]string{
					"preset-e2e-aws-alb-ingress-controller": "true",
				},

				Env: []v1.EnvVar{
					{Name: "AWSTESTER_EKS_KUBETEST_EMBEDDED_BINARY", Value: cfg.AWSTESTER_EKS_KUBETEST_EMBEDDED_BINARY},

					{Name: "AWSTESTER_EKS_WAIT_BEFORE_DOWN", Value: cfg.AWSTESTER_EKS_WAIT_BEFORE_DOWN},
					{Name: "AWSTESTER_EKS_DOWN", Value: cfg.AWSTESTER_EKS_DOWN},

					{Name: "AWSTESTER_EKS_ENABLE_WORKER_NODE_HA", Value: cfg.AWSTESTER_EKS_ENABLE_WORKER_NODE_HA},
					{Name: "AWSTESTER_EKS_ENABLE_NODE_SSH", Value: cfg.AWSTESTER_EKS_ENABLE_NODE_SSH},

					{Name: "AWSTESTER_EKS_AWSTESTER_IMAGE", Value: cfg.AWSTESTER_EKS_AWSTESTER_IMAGE},

					{Name: "AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE", Value: cfg.AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE},
					{Name: "AWSTESTER_EKS_WORKER_NODE_ASG_MIN", Value: cfg.AWSTESTER_EKS_WORKER_NODE_ASG_MIN},
					{Name: "AWSTESTER_EKS_WORKER_NODE_ASG_MAX", Value: cfg.AWSTESTER_EKS_WORKER_NODE_ASG_MAX},

					{Name: "AWSTESTER_EKS_LOG_DEBUG", Value: cfg.AWSTESTER_EKS_LOG_DEBUG},
					{Name: "AWSTESTER_EKS_LOG_ACCESS", Value: cfg.AWSTESTER_EKS_LOG_ACCESS},
					{Name: "AWSTESTER_EKS_UPLOAD_AWS_TESTER_LOGS", Value: cfg.AWSTESTER_EKS_UPLOAD_AWS_TESTER_LOGS},
					{Name: "AWSTESTER_EKS_ALB_UPLOAD_TESTER_LOGS", Value: cfg.AWSTESTER_EKS_ALB_UPLOAD_TESTER_LOGS},
					{Name: "AWSTESTER_EKS_UPLOAD_WORKER_NODE_LOGS", Value: cfg.AWSTESTER_EKS_UPLOAD_WORKER_NODE_LOGS},

					{Name: "AWSTESTER_EKS_ALB_ENABLE", Value: cfg.AWSTESTER_EKS_ALB_ENABLE},

					{Name: "AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE", Value: cfg.AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE},

					{Name: "AWSTESTER_EKS_ALB_TARGET_TYPE", Value: cfg.AWSTESTER_EKS_ALB_TARGET_TYPE},
					{Name: "AWSTESTER_EKS_ALB_TEST_MODE", Value: cfg.AWSTESTER_EKS_ALB_TEST_MODE},

					{Name: "AWSTESTER_EKS_ALB_TEST_SCALABILITY", Value: cfg.AWSTESTER_EKS_ALB_TEST_SCALABILITY},
					{Name: "AWSTESTER_EKS_ALB_TEST_METRICS", Value: cfg.AWSTESTER_EKS_ALB_TEST_METRICS},
					{Name: "AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS", Value: cfg.AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS},
					{Name: "AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES", Value: cfg.AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES},
					{Name: "AWSTESTER_EKS_ALB_TEST_CLIENTS", Value: cfg.AWSTESTER_EKS_ALB_TEST_CLIENTS},
					{Name: "AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS", Value: cfg.AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS},
					{Name: "AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE", Value: cfg.AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE},
					{Name: "AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD", Value: cfg.AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD},
					{Name: "AWSTESTER_EKS_ALB_TEST_EXPECT_QPS", Value: cfg.AWSTESTER_EKS_ALB_TEST_EXPECT_QPS},

					{Name: "AWS_SHARED_CREDENTIALS_FILE", Value: "/etc/aws-cred-awstester/aws-cred-awstester"},
				},

				VolumeMounts: []v1.VolumeMount{
					{
						Name:      "aws-cred-awstester",
						MountPath: "/etc/aws-cred-awstester",
						ReadOnly:  true,
					},
				},

				Volumes: []v1.Volume{
					{
						Name: "aws-cred-awstester",
						VolumeSource: v1.VolumeSource{
							Secret: &v1.SecretVolumeSource{
								SecretName: "aws-cred-awstester",
							},
						},
					},
				},
			},
		},

		Periodics: []config.Periodic{
			{
				Name: "aws-alb-ingress-controller-periodics",
				Labels: map[string]string{
					"preset-dind-enabled":                   "true",
					"preset-e2e-aws-alb-ingress-controller": "true",
				},

				Agent:    "kubernetes",
				Interval: "72h",
				Spec: &v1.PodSpec{
					Containers: []v1.Container{
						{
							Image: "alpine",
							Command: []string{
								"/bin/echo",
								"aws-alb-ingress-controller-periodics",
							},
						},
					},
				},
			},
		},

		Postsubmits: map[string][]config.Postsubmit{
			"gyuho/aws-alb-ingress-controller": {
				{
					Name: "aws-alb-ingress-controller-postsubmit",
					Labels: map[string]string{
						"preset-dind-enabled":                   "true",
						"preset-e2e-aws-alb-ingress-controller": "true",
					},

					Agent: "kubernetes",
					Spec: &v1.PodSpec{
						Containers: []v1.Container{
							{
								Image: "alpine",
								Command: []string{
									"/bin/echo",
									"aws-alb-ingress-controller-postsubmit",
								},
							},
						},
					},
				},
			},
		},

		Presubmits: map[string][]config.Presubmit{
			"gyuho/aws-alb-ingress-controller": {
				{
					Name: "pull-ingress-aws-alb-build",
					Labels: map[string]string{
						"preset-dind-enabled":                   "true",
						"preset-e2e-aws-alb-ingress-controller": "true",
					},

					Agent:   "kubernetes",
					Context: "test-presubmit",

					Trigger:      "(?m)^/test pull-ingress-aws-alb-build",
					RerunCommand: "/test pull-ingress-aws-alb-build",

					AlwaysRun:  true,
					SkipReport: false,

					Spec: &v1.PodSpec{
						Containers: []v1.Container{
							{
								// TODO: enable after open-source
								// "gcr.io/k8s-testimages/kubekins-e2e:v20181005-fd9cfb8b0-master"
								//
								// use custom built image to include "wrk", "awstester", etc.
								// e.g. 607362164682.dkr.ecr.us-west-2.amazonaws.com/kubekins-e2e:ade682b5fc04
								//
								// Image: cfg.AWSTESTER_EKS_KUBEKINS_E2E_IMAGE,
								// Args: []string{
								// 	"--repo=github.com/gyuho/$(REPO_NAME)=$(PULL_REFS)",
								// 	"--root=/go/src",
								// 	// "--upload=gs://kubernetes-jenkins/pr-logs",
								// 	"--scenario=execute",
								// 	"--",
								// 	"./test/build.sh",
								// },

								Image: "alpine",
								Command: []string{
									"/bin/echo",
									"pull-ingress-aws-alb-build",
								},

								// TODO: to build docker in docker
								SecurityContext: &v1.SecurityContext{
									Privileged: aws.Bool(true),
								},
							},
						},
					},

					RunAfterSuccess: []config.Presubmit{
						{
							Name: "pull-ingress-aws-alb-e2e",
							Labels: map[string]string{
								"preset-dind-enabled":                   "true",
								"preset-e2e-aws-alb-ingress-controller": "true",
							},

							Agent:   "kubernetes",
							Context: "test-presubmit",

							Trigger:      "(?m)^/test pull-ingress-aws-alb-e2e",
							RerunCommand: "/test pull-ingress-aws-alb-e2e",

							AlwaysRun:  true,
							SkipReport: false,

							Spec: &v1.PodSpec{
								Containers: []v1.Container{
									{
										Image: cfg.AWSTESTER_EKS_AWSTESTER_IMAGE,
										Command: []string{
											"tests/alb-e2e.sh",
										},

										// TODO: to build docker in docker
										SecurityContext: &v1.SecurityContext{
											Privileged: aws.Bool(true),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	d, err := gyaml.Marshal(jcfg)
	if err != nil {
		return "", err
	}
	return string(d), nil
}
