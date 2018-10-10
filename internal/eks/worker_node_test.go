package eks

import (
	"os"
	"testing"
)

func Test_writeConfigMapNodeAuth(t *testing.T) {
	p, err := writeConfigMapNodeAuth("sample")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(p)
	os.RemoveAll(p)
}

func Test_countReadyNodesFromKubectlGetNodesOutputYAML(t *testing.T) {
	v, err := countReadyNodesFromKubectlGetNodesOutputYAML([]byte(testKubectlGetNodesOutput))
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("expected 2 ready nodes, got %d", v)
	}
}

const testKubectlGetNodesOutput = `---
apiVersion: v1
items:
- apiVersion: v1
  kind: Node
  metadata:
    annotations:
      node.alpha.kubernetes.io/ttl: "0"
      volumes.kubernetes.io/controller-managed-attach-detach: "true"
    creationTimestamp: 2018-09-14T03:32:53Z
    labels:
      beta.kubernetes.io/arch: amd64
      beta.kubernetes.io/instance-type: m5.large
      beta.kubernetes.io/os: linux
      failure-domain.beta.kubernetes.io/region: us-west-2
      failure-domain.beta.kubernetes.io/zone: us-west-2c
      kubernetes.io/hostname: ip-192-168-192-77.us-west-2.compute.internal
    name: ip-192-168-192-77.us-west-2.compute.internal
    namespace: ""
    resourceVersion: "283586"
    selfLink: /api/v1/nodes/ip-192-168-192-77.us-west-2.compute.internal
    uid: dd4dca55-b7ce-11e8-9beb-02147d49ffd4
  spec:
    externalID: i-0eab6efc66ff5d4c7
    providerID: aws:///us-west-2c/i-0eab6efc66ff5d4c7
  status:
    addresses:
    - address: 192.168.192.77
      type: InternalIP
    - address: ip-192-168-192-77.us-west-2.compute.internal
      type: Hostname
    allocatable:
      cpu: "2"
      ephemeral-storage: "48307038948"
      hugepages-1Gi: "0"
      hugepages-2Mi: "0"
      memory: 7765052Ki
      pods: "29"
    capacity:
      cpu: "2"
      ephemeral-storage: 52416492Ki
      hugepages-1Gi: "0"
      hugepages-2Mi: "0"
      memory: 7867452Ki
      pods: "29"
    conditions:
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has sufficient disk space available
      reason: KubeletHasSufficientDisk
      status: "False"
      type: OutOfDisk
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has sufficient memory available
      reason: KubeletHasSufficientMemory
      status: "False"
      type: MemoryPressure
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has no disk pressure
      reason: KubeletHasNoDiskPressure
      status: "False"
      type: DiskPressure
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has sufficient PID available
      reason: KubeletHasSufficientPID
      status: "False"
      type: PIDPressure
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:33:14Z
      message: kubelet is posting ready status
      reason: KubeletReady
      status: "True"
      type: Ready
    daemonEndpoints:
      kubeletEndpoint:
        Port: 10250
    images:
    - names:
      - 607362164682.dkr.ecr.us-west-2.amazonaws.com/eks-test-status-upstream@sha256:52aac60da91c77676543243a1846dbc7fa7da114ad6ee79aacb45afbf8588555
      - 607362164682.dkr.ecr.us-west-2.amazonaws.com/eks-test-status-upstream:v0.0.1
      sizeBytes: 836779711
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni@sha256:db4efc51d5ed4ecaca3f5775655722be57825702e37df1bdedca8ce297289870
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:1.1.0
      sizeBytes: 391689144
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-proxy@sha256:76927fb03bd6b37be4330c356e95bcac16ee6961a12da7b7e6ffa50db376438c
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-proxy:v1.10.3
      sizeBytes: 96933403
    - names:
      - 607362164682.dkr.ecr.us-west-2.amazonaws.com/alb@sha256:8fc52c66f69cede2aff8bf6ebb15588ddd1057ffed85d6d93610c1d9e2b0b142
      - quay.io/coreos/alb-ingress-controller:1.0-beta.7
      sizeBytes: 61979444
    - names:
      - gcr.io/k8s-prow/hook@sha256:c3a401520c052fbb3810fb91c2d136fef98e04d97b2ae46fd9160f544f8a286c
      - gcr.io/k8s-prow/hook:v20180910-f829443bd
      sizeBytes: 47919430
    - names:
      - gcr.io/k8s-prow/plank@sha256:95b59ae4cc9d6315bab3ac932aa10b81aaa5430c70b0f643f9fe8a32bbe1bcfb
      - gcr.io/k8s-prow/plank:v20180910-f829443bd
      sizeBytes: 21863344
    - names:
      - gcr.io/k8s-prow/sinker@sha256:d0c800173bae40f082c8fe751d65c0287bf147934dc26ab3ced7ac42c04703ae
      - gcr.io/k8s-prow/sinker:v20180910-f829443bd
      sizeBytes: 20016566
    - names:
      - alpine@sha256:621c2f39f8133acb8e64023a94dbdf0d5ca81896102b9e57c0dc184cadaf5528
      - alpine:latest
      sizeBytes: 4413370
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/pause-amd64@sha256:bea77c323c47f7b573355516acf927691182d1333333d1f41b7544012fab7adf
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/pause-amd64:3.1
      sizeBytes: 742472
    nodeInfo:
      architecture: amd64
      bootID: 9988356f-97d2-4af2-bfce-62e589010dd8
      containerRuntimeVersion: docker://17.6.2
      kernelVersion: 4.14.62-70.117.amzn2.x86_64
      kubeProxyVersion: v1.10.3
      kubeletVersion: v1.10.3
      machineID: d392e73215034bffb2cdb5478a98623a
      operatingSystem: linux
      osImage: Amazon Linux 2
      systemUUID: EC271A9F-A728-91BD-F7D3-7C07D06D48AF
- apiVersion: v1
  kind: Node
  metadata:
    annotations:
      node.alpha.kubernetes.io/ttl: "0"
      volumes.kubernetes.io/controller-managed-attach-detach: "true"
    creationTimestamp: 2018-09-14T03:32:53Z
    labels:
      beta.kubernetes.io/arch: amd64
      beta.kubernetes.io/instance-type: m5.large
      beta.kubernetes.io/os: linux
      failure-domain.beta.kubernetes.io/region: us-west-2
      failure-domain.beta.kubernetes.io/zone: us-west-2a
      kubernetes.io/hostname: ip-192-168-87-77.us-west-2.compute.internal
    name: ip-192-168-87-77.us-west-2.compute.internal
    namespace: ""
    resourceVersion: "283585"
    selfLink: /api/v1/nodes/ip-192-168-87-77.us-west-2.compute.internal
    uid: dd572714-b7ce-11e8-9beb-02147d49ffd4
  spec:
    externalID: i-0679f6a94ad595169
    providerID: aws:///us-west-2a/i-0679f6a94ad595169
  status:
    addresses:
    - address: 192.168.87.77
      type: InternalIP
    - address: ip-192-168-87-77.us-west-2.compute.internal
      type: Hostname
    allocatable:
      cpu: "2"
      ephemeral-storage: "48307038948"
      hugepages-1Gi: "0"
      hugepages-2Mi: "0"
      memory: 7765052Ki
      pods: "29"
    capacity:
      cpu: "2"
      ephemeral-storage: 52416492Ki
      hugepages-1Gi: "0"
      hugepages-2Mi: "0"
      memory: 7867452Ki
      pods: "29"
    conditions:
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has sufficient disk space available
      reason: KubeletHasSufficientDisk
      status: "False"
      type: OutOfDisk
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has sufficient memory available
      reason: KubeletHasSufficientMemory
      status: "False"
      type: MemoryPressure
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has no disk pressure
      reason: KubeletHasNoDiskPressure
      status: "False"
      type: DiskPressure
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:32:49Z
      message: kubelet has sufficient PID available
      reason: KubeletHasSufficientPID
      status: "False"
      type: PIDPressure
    - lastHeartbeatTime: 2018-09-16T06:55:46Z
      lastTransitionTime: 2018-09-14T03:33:13Z
      message: kubelet is posting ready status
      reason: KubeletReady
      status: "True"
      type: Ready
    daemonEndpoints:
      kubeletEndpoint:
        Port: 10250
    images:
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni@sha256:db4efc51d5ed4ecaca3f5775655722be57825702e37df1bdedca8ce297289870
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon-k8s-cni:1.1.0
      sizeBytes: 391689144
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-proxy@sha256:76927fb03bd6b37be4330c356e95bcac16ee6961a12da7b7e6ffa50db376438c
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-proxy:v1.10.3
      sizeBytes: 96933403
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-dns/kube-dns@sha256:7f2b2001f6571c4c44fc18005cee662254c3d931447a57cdbea65aaa4ae2dc16
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-dns/kube-dns:1.14.10
      sizeBytes: 49549457
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-dns/sidecar@sha256:5ff6aa8810bebe526ac20c752d7bd67c44f05fcb6f8759982fb782488a657110
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-dns/sidecar:1.14.10
      sizeBytes: 41635309
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-dns/dnsmasq-nanny@sha256:acdfb44e37405dc9cfa25dca2a07ed97f35d0d77e34f18d8348475f9c181ad09
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/kube-dns/dnsmasq-nanny:1.14.10
      sizeBytes: 40372149
    - names:
      - gcr.io/k8s-prow/deck@sha256:7e0865ebc83f0bbe64000c89c26ce23aad0c08d93bc6093ce279bfa698640af3
      - gcr.io/k8s-prow/deck:v20180910-f829443bd
      sizeBytes: 23436749
    - names:
      - gcr.io/k8s-prow/horologium@sha256:a51f91b78407a9f34d1b0bc3b3572ef0b8520a515f64eab4a4de097a8a68d592
      - gcr.io/k8s-prow/horologium:v20180910-f829443bd
      sizeBytes: 20040216
    - names:
      - alpine@sha256:621c2f39f8133acb8e64023a94dbdf0d5ca81896102b9e57c0dc184cadaf5528
      - alpine:latest
      sizeBytes: 4413370
    - names:
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/pause-amd64@sha256:bea77c323c47f7b573355516acf927691182d1333333d1f41b7544012fab7adf
      - 602401143452.dkr.ecr.us-west-2.amazonaws.com/eks/pause-amd64:3.1
      sizeBytes: 742472
    nodeInfo:
      architecture: amd64
      bootID: 654aed87-05e7-4911-b936-3387a85d6006
      containerRuntimeVersion: docker://17.6.2
      kernelVersion: 4.14.62-70.117.amzn2.x86_64
      kubeProxyVersion: v1.10.3
      kubeletVersion: v1.10.3
      machineID: d392e73215034bffb2cdb5478a98623a
      operatingSystem: linux
      osImage: Amazon Linux 2
      systemUUID: EC2A0C92-8B2B-A3B0-6909-53CA040F98D8
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""

`
