package alb

import "testing"

func Test_findReadyPodsFromKubectlGetPodsOutputYAML(t *testing.T) {
	if !findReadyPodsFromKubectlGetPodsOutputYAML([]byte(sampleGetPodsOutput), "deck") {
		t.Fatal("expected 'deck' Pod ready")
	}
}

const sampleGetPodsOutput = `
apiVersion: v1
items:
- apiVersion: v1
  kind: Pod
  metadata:
    creationTimestamp: 2018-09-16T22:05:49Z
    generateName: alb-ingress-controller-7bd4bdfcdc-
    labels:
      app: alb-ingress-controller
      pod-template-hash: "3680689787"
    name: alb-ingress-controller-7bd4bdfcdc-fsjbn
    namespace: kube-system
    ownerReferences:
    - apiVersion: extensions/v1beta1
      blockOwnerDeletion: true
      controller: true
      kind: ReplicaSet
      name: alb-ingress-controller-7bd4bdfcdc
      uid: ab7c671e-b9fc-11e8-852e-02d628d39cc2
    resourceVersion: "2434"
    selfLink: /api/v1/namespaces/kube-system/pods/alb-ingress-controller-7bd4bdfcdc-fsjbn
    uid: ab7ded17-b9fc-11e8-852e-02d628d39cc2
  spec:
    containers:
    - args:
      - /server
      - --ingress-class=alb
      - --cluster-name=EKS-PROW-CLUSTER
      - --aws-max-retries=20
      - --healthz-port=10254
      env:
      - name: AWS_REGION
        value: us-west-2
      - name: AWS_DEBUG
        value: "false"
      - name: POD_NAME
        valueFrom:
          fieldRef:
            apiVersion: v1
            fieldPath: metadata.name
      - name: POD_NAMESPACE
        valueFrom:
          fieldRef:
            apiVersion: v1
            fieldPath: metadata.namespace
      - name: AWS_SHARED_CREDENTIALS_FILE
        value: /etc/aws-cred-awstester/aws-cred-awstester
      image: quay.io/coreos/alb-ingress-controller:1.0-beta.7
      imagePullPolicy: Always
      livenessProbe:
        failureThreshold: 3
        httpGet:
          path: /healthz
          port: 10254
          scheme: HTTP
        initialDelaySeconds: 10
        periodSeconds: 60
        successThreshold: 1
        timeoutSeconds: 1
      name: server
      ports:
      - containerPort: 10254
        protocol: TCP
      readinessProbe:
        failureThreshold: 3
        httpGet:
          path: /healthz
          port: 10254
          scheme: HTTP
        initialDelaySeconds: 10
        periodSeconds: 60
        successThreshold: 1
        timeoutSeconds: 30
      resources: {}
      terminationMessagePath: /dev/termination-log
      terminationMessagePolicy: File
      volumeMounts:
      - mountPath: /etc/aws-cred-awstester
        name: aws-cred-awstester
        readOnly: true
      - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
        name: alb-ingress-token-h5q44
        readOnly: true
    dnsPolicy: ClusterFirst
    nodeName: ip-192-168-221-104.us-west-2.compute.internal
    restartPolicy: Always
    schedulerName: default-scheduler
    securityContext: {}
    serviceAccount: alb-ingress
    serviceAccountName: alb-ingress
    terminationGracePeriodSeconds: 30
    tolerations:
    - effect: NoExecute
      key: node.kubernetes.io/not-ready
      operator: Exists
      tolerationSeconds: 300
    - effect: NoExecute
      key: node.kubernetes.io/unreachable
      operator: Exists
      tolerationSeconds: 300
    volumes:
    - name: aws-cred-awstester
      secret:
        defaultMode: 420
        secretName: aws-cred-awstester
    - name: alb-ingress-token-h5q44
      secret:
        defaultMode: 420
        secretName: alb-ingress-token-h5q44
  status:
    conditions:
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T22:05:49Z
      status: "True"
      type: Initialized
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T22:06:31Z
      status: "True"
      type: Ready
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T22:05:49Z
      status: "True"
      type: PodScheduled
    containerStatuses:
    - containerID: docker://03681e2f0f1fc35c69a2090de6ed3c695d9e96bc4e71dc02b389e3b42a2b4646
      image: quay.io/coreos/alb-ingress-controller:1.0-beta.7
      imageID: docker-pullable://607362164682.dkr.ecr.us-west-2.amazonaws.com/alb@sha256:8fc52c66f69cede2aff8bf6ebb15588ddd1057ffed85d6d93610c1d9e2b0b142
      lastState: {}
      name: server
      ready: true
      restartCount: 0
      state:
        running:
          startedAt: 2018-09-16T22:05:52Z
    hostIP: 192.168.221.104
    phase: Running
    podIP: 192.168.196.80
    qosClass: BestEffort
    startTime: 2018-09-16T22:05:49Z
- apiVersion: v1
  kind: Pod
  metadata:
    annotations:
      scheduler.alpha.kubernetes.io/critical-pod: ""
    creationTimestamp: 2018-09-16T21:52:16Z
    generateName: aws-node-
    labels:
      controller-revision-hash: "3893463860"
      k8s-app: aws-node
      pod-template-generation: "1"
    name: aws-node-7mwqc
    namespace: kube-system
    ownerReferences:
    - apiVersion: apps/v1
      blockOwnerDeletion: true
      controller: true
      kind: DaemonSet
      name: aws-node
      uid: 75fdeaba-b9f9-11e8-852e-02d628d39cc2
    resourceVersion: "975"
    selfLink: /api/v1/namespaces/kube-system/pods/aws-node-7mwqc
    uid: c6d3f32d-b9fa-11e8-852e-02d628d39cc2
  spec:
    containers:
    - env:
      - name: AWS_VPC_K8S_CNI_LOGLEVEL
        value: DEBUG
      - name: MY_NODE_NAME
        valueFrom:
          fieldRef:
            apiVersion: v1
            fieldPath: spec.nodeName
      image: 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:1.1.0
      imagePullPolicy: IfNotPresent
      name: aws-node
      resources:
        requests:
          cpu: 10m
      securityContext:
        privileged: true
      terminationMessagePath: /dev/termination-log
      terminationMessagePolicy: File
      volumeMounts:
      - mountPath: /host/opt/cni/bin
        name: cni-bin-dir
      - mountPath: /host/etc/cni/net.d
        name: cni-net-dir
      - mountPath: /host/var/log
        name: log-dir
      - mountPath: /var/run/docker.sock
        name: dockersock
      - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
        name: aws-node-token-mkz62
        readOnly: true
    dnsPolicy: ClusterFirst
    hostNetwork: true
    nodeName: ip-192-168-105-121.us-west-2.compute.internal
    restartPolicy: Always
    schedulerName: default-scheduler
    securityContext: {}
    serviceAccount: aws-node
    serviceAccountName: aws-node
    terminationGracePeriodSeconds: 30
    tolerations:
    - operator: Exists
    - effect: NoExecute
      key: node.kubernetes.io/not-ready
      operator: Exists
    - effect: NoExecute
      key: node.kubernetes.io/unreachable
      operator: Exists
    - effect: NoSchedule
      key: node.kubernetes.io/disk-pressure
      operator: Exists
    - effect: NoSchedule
      key: node.kubernetes.io/memory-pressure
      operator: Exists
    volumes:
    - hostPath:
        path: /opt/cni/bin
        type: ""
      name: cni-bin-dir
    - hostPath:
        path: /etc/cni/net.d
        type: ""
      name: cni-net-dir
    - hostPath:
        path: /var/log
        type: ""
      name: log-dir
    - hostPath:
        path: /var/run/docker.sock
        type: ""
      name: dockersock
    - name: aws-node-token-mkz62
      secret:
        defaultMode: 420
        secretName: aws-node-token-mkz62
  status:
    conditions:
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T21:52:16Z
      status: "True"
      type: Initialized
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T21:52:30Z
      status: "True"
      type: Ready
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T21:52:16Z
      status: "True"
      type: PodScheduled
    containerStatuses:
    - containerID: docker://6a433a3b617b871559184e2168d86b06bc3b1af2cb0c457f6f131992974e0112
      image: 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:1.1.0
      imageID: docker-pullable://602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni@sha256:db4efc51d5ed4ecaca3f5775655722be57825702e37df1bdedca8ce297289870
      lastState: {}
      name: aws-node
      ready: true
      restartCount: 0
      state:
        running:
          startedAt: 2018-09-16T21:52:30Z
    hostIP: 192.168.105.121
    phase: Running
    podIP: 192.168.105.121
    qosClass: Burstable
    startTime: 2018-09-16T21:52:16Z
- apiVersion: v1
  kind: Pod
  metadata:
    annotations:
      scheduler.alpha.kubernetes.io/critical-pod: ""
    creationTimestamp: 2018-09-16T21:52:16Z
    generateName: aws-node-
    labels:
      controller-revision-hash: "3893463860"
      k8s-app: aws-node
      pod-template-generation: "1"
    name: aws-node-vv88p
    namespace: kube-system
    ownerReferences:
    - apiVersion: apps/v1
      blockOwnerDeletion: true
      controller: true
      kind: DaemonSet
      name: aws-node
      uid: 75fdeaba-b9f9-11e8-852e-02d628d39cc2
    resourceVersion: "977"
    selfLink: /api/v1/namespaces/kube-system/pods/aws-node-vv88p
    uid: c6fc1000-b9fa-11e8-852e-02d628d39cc2
  spec:
    containers:
    - env:
      - name: AWS_VPC_K8S_CNI_LOGLEVEL
        value: DEBUG
      - name: MY_NODE_NAME
        valueFrom:
          fieldRef:
            apiVersion: v1
            fieldPath: spec.nodeName
      image: 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:1.1.0
      imagePullPolicy: IfNotPresent
      name: aws-node
      resources:
        requests:
          cpu: 10m
      securityContext:
        privileged: true
      terminationMessagePath: /dev/termination-log
      terminationMessagePolicy: File
      volumeMounts:
      - mountPath: /host/opt/cni/bin
        name: cni-bin-dir
      - mountPath: /host/etc/cni/net.d
        name: cni-net-dir
      - mountPath: /host/var/log
        name: log-dir
      - mountPath: /var/run/docker.sock
        name: dockersock
      - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
        name: aws-node-token-mkz62
        readOnly: true
    dnsPolicy: ClusterFirst
    hostNetwork: true
    nodeName: ip-192-168-221-104.us-west-2.compute.internal
    restartPolicy: Always
    schedulerName: default-scheduler
    securityContext: {}
    serviceAccount: aws-node
    serviceAccountName: aws-node
    terminationGracePeriodSeconds: 30
    tolerations:
    - operator: Exists
    - effect: NoExecute
      key: node.kubernetes.io/not-ready
      operator: Exists
    - effect: NoExecute
      key: node.kubernetes.io/unreachable
      operator: Exists
    - effect: NoSchedule
      key: node.kubernetes.io/disk-pressure
      operator: Exists
    - effect: NoSchedule
      key: node.kubernetes.io/memory-pressure
      operator: Exists
    volumes:
    - hostPath:
        path: /opt/cni/bin
        type: ""
      name: cni-bin-dir
    - hostPath:
        path: /etc/cni/net.d
        type: ""
      name: cni-net-dir
    - hostPath:
        path: /var/log
        type: ""
      name: log-dir
    - hostPath:
        path: /var/run/docker.sock
        type: ""
      name: dockersock
    - name: aws-node-token-mkz62
      secret:
        defaultMode: 420
        secretName: aws-node-token-mkz62
  status:
    conditions:
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T21:52:16Z
      status: "True"
      type: Initialized
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T21:52:30Z
      status: "True"
      type: Ready
    - lastProbeTime: null
      lastTransitionTime: 2018-09-16T21:52:16Z
      status: "True"
      type: PodScheduled
    containerStatuses:
    - containerID: docker://f712f25bb6998c2d195496f8f830ac6c7fc9097f1adf50627c22e70e9a2aa129
      image: 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:1.1.0
      imageID: docker-pullable://602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni@sha256:db4efc51d5ed4ecaca3f5775655722be57825702e37df1bdedca8ce297289870
      lastState: {}
      name: aws-node
      ready: true
      restartCount: 0
      state:
        running:
          startedAt: 2018-09-16T21:52:30Z
    hostIP: 192.168.221.104
    phase: Running
    podIP: 192.168.221.104
    qosClass: Burstable
    startTime: 2018-09-16T21:52:16Z

kind: List
metadata:
  resourceVersion: ""
  selfLink: ""

`
