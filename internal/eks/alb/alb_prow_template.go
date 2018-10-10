package alb

// TODO: replace it with flags once open-sourced?

// TODO: use struct rather than template
const albProwTempl = `
periodics:
- name: aws-alb-ingress-controller-periodics
  agent: kubernetes
  interval: 24h

  spec:
    containers:
    - image: alpine
      command:
      - /bin/echo
      - aws-alb-ingress-controller-periodics



postsubmits:
  gyuho/aws-alb-ingress-controller:
  - name: aws-alb-ingress-controller-postsubmit
    agent: kubernetes

    spec:
      containers:
      - image: alpine
        command:
        - /bin/echo
        - aws-alb-ingress-controller-postsubmit



presubmits:
  gyuho/aws-alb-ingress-controller:
  - name: ci-ingress-aws-alb-e2e
    agent: kubernetes
    trigger: "(?m)^/test ci-ingress-aws-alb-e2e"
    rerun_command: "/test ci-ingress-aws-alb-e2e"
    context: test-presubmit
    always_run: true
    skip_report: false

    spec:
      containers:
      - image: {{ .AWSTESTER_EKS_AWSTESTER_IMAGE }}

        # TODO: to build docker in docker
        securityContext:
          privileged: true

        command:
        - tests/alb-e2e.sh

        env:
        - name: GINKGO_TIMEOUT
          value: {{ .GINKGO_TIMEOUT }}
        - name: GINKGO_VERBOSE
          value: {{ .GINKGO_VERBOSE }}

        - name: AWSTESTER_EKS_EMBEDDED
          value: {{ .AWSTESTER_EKS_EMBEDDED }}
        - name: AWSTESTER_EKS_WAIT_BEFORE_DOWN
          value: {{ .AWSTESTER_EKS_WAIT_BEFORE_DOWN }}
        - name: AWSTESTER_EKS_DOWN
          value: {{ .AWSTESTER_EKS_DOWN }}

        - name: AWSTESTER_EKS_AWSTESTER_IMAGE
          value: {{ .AWSTESTER_EKS_AWSTESTER_IMAGE }}

        - name: AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE
          value: {{ .AWSTESTER_EKS_WORKER_NODE_INSTANCE_TYPE }}
        - name: AWSTESTER_EKS_WORKER_NODE_ASG_MIN
          value: {{ .AWSTESTER_EKS_WORKER_NODE_ASG_MIN }}
        - name: AWSTESTER_EKS_WORKER_NODE_ASG_MAX
          value: {{ .AWSTESTER_EKS_WORKER_NODE_ASG_MAX }}

        - name: AWSTESTER_EKS_ALB_ENABLE
          value: {{ .AWSTESTER_EKS_ALB_ENABLE }}
        - name: AWSTESTER_EKS_ALB_ENABLE_SCALABILITY_TEST
          value: {{ .AWSTESTER_EKS_ALB_ENABLE_SCALABILITY_TEST }}

        - name: AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE
          value: {{ .AWSTESTER_EKS_ALB_ALB_INGRESS_CONTROLLER_IMAGE }}

        - name: AWSTESTER_EKS_ALB_TARGET_TYPE
          value: {{ .AWSTESTER_EKS_ALB_TARGET_TYPE }}
        - name: AWSTESTER_EKS_ALB_TEST_MODE
          value: {{ .AWSTESTER_EKS_ALB_TEST_MODE }}

        - name: AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS
          value: {{ .AWSTESTER_EKS_ALB_TEST_SERVER_REPLICAS }}
        - name: AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES
          value: {{ .AWSTESTER_EKS_ALB_TEST_SERVER_ROUTES }}
        - name: AWSTESTER_EKS_ALB_TEST_CLIENTS
          value: {{ .AWSTESTER_EKS_ALB_TEST_CLIENTS }}
        - name: AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS
          value: {{ .AWSTESTER_EKS_ALB_TEST_CLIENT_REQUESTS }}
        - name: AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE
          value: {{ .AWSTESTER_EKS_ALB_TEST_RESPONSE_SIZE }}
        - name: AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD
          value: {{ .AWSTESTER_EKS_ALB_TEST_CLIENT_ERROR_THRESHOLD }}
        - name: AWSTESTER_EKS_ALB_TEST_EXPECT_QPS
          value: {{ .AWSTESTER_EKS_ALB_TEST_EXPECT_QPS }}

        - name: AWS_SHARED_CREDENTIALS_FILE
          value: /etc/aws-cred-awstester/aws-cred-awstester

        volumeMounts:
        - name: aws-cred-awstester
          mountPath: /etc/aws-cred-awstester
          readOnly: true

      volumes:
      - name: aws-cred-awstester
        secret:
          secretName: aws-cred-awstester

`
