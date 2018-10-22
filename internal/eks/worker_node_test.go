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

func Test_kubectlGetNodes(t *testing.T) {
	ns, err := kubectlGetNodes([]byte(testKubectlGetNodesOutputJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(ns.Items) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(ns.Items))
	}
	rn := countReadyNodes(ns)
	if rn != 2 {
		t.Fatalf("expected 2 ready nodes, got %d", rn)
	}
}

const testKubectlGetNodesOutputJSON = `
{
  "apiVersion": "v1",
  "items": [
      {
          "apiVersion": "v1",
          "kind": "Node",
          "metadata": {
              "annotations": {
                  "node.alpha.kubernetes.io/ttl": "0",
                  "volumes.kubernetes.io/controller-managed-attach-detach": "true"
              }
          },
          "spec": {
              "externalID": "i-052eb2cdfb10e0efe",
              "providerID": "aws:///us-west-2c/i-052eb2cdfb10e0efe"
          },
          "status": {
              "addresses": [
                  {
                      "address": "192.168.255.108",
                      "type": "InternalIP"
                  },
                  {
                      "address": "ip-192-168-255-108.us-west-2.compute.internal",
                      "type": "Hostname"
                  }
              ],
              "allocatable": {
                  "cpu": "2",
                  "ephemeral-storage": "48307038948",
                  "hugepages-1Gi": "0",
                  "hugepages-2Mi": "0",
                  "memory": "7765052Ki",
                  "pods": "29"
              },
              "capacity": {
                  "cpu": "2",
                  "ephemeral-storage": "52416492Ki",
                  "hugepages-1Gi": "0",
                  "hugepages-2Mi": "0",
                  "memory": "7867452Ki",
                  "pods": "29"
              },
              "conditions": [
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:12Z",
                      "message": "kubelet has sufficient disk space available",
                      "reason": "KubeletHasSufficientDisk",
                      "status": "False",
                      "type": "OutOfDisk"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:12Z",
                      "message": "kubelet has sufficient memory available",
                      "reason": "KubeletHasSufficientMemory",
                      "status": "False",
                      "type": "MemoryPressure"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:12Z",
                      "message": "kubelet has no disk pressure",
                      "reason": "KubeletHasNoDiskPressure",
                      "status": "False",
                      "type": "DiskPressure"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:12Z",
                      "message": "kubelet has sufficient PID available",
                      "reason": "KubeletHasSufficientPID",
                      "status": "False",
                      "type": "PIDPressure"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:32Z",
                      "message": "kubelet is posting ready status",
                      "reason": "KubeletReady",
                      "status": "True",
                      "type": "Ready"
                  }
              ],
              "daemonEndpoints": {
                  "kubeletEndpoint": {
                      "Port": 10250
                  }
              },
              "nodeInfo": {
                  "architecture": "amd64",
                  "bootID": "c0a8855a-d98b-4a33-8140-c4544800cef7",
                  "containerRuntimeVersion": "docker://17.6.2",
                  "kernelVersion": "4.14.62-70.117.amzn2.x86_64",
                  "kubeProxyVersion": "v1.10.3",
                  "kubeletVersion": "v1.10.3",
                  "machineID": "ec2222be1f9fd6911fb70f4e3106bda5",
                  "operatingSystem": "linux",
                  "osImage": "Amazon Linux 2",
                  "systemUUID": "EC2222BE-1F9F-D691-1FB7-0F4E3106BDA5"
              }
          }
      },
      {
          "apiVersion": "v1",
          "kind": "Node",
          "metadata": {
              "annotations": {
                  "node.alpha.kubernetes.io/ttl": "0",
                  "volumes.kubernetes.io/controller-managed-attach-detach": "true"
              },
              "creationTimestamp": "2018-10-14T03:16:16Z",
              "labels": {
                  "beta.kubernetes.io/arch": "amd64",
                  "beta.kubernetes.io/instance-type": "m5.large",
                  "beta.kubernetes.io/os": "linux",
                  "failure-domain.beta.kubernetes.io/region": "us-west-2",
                  "failure-domain.beta.kubernetes.io/zone": "us-west-2a",
                  "kubernetes.io/hostname": "ip-192-168-78-24.us-west-2.compute.internal"
              },
              "name": "ip-192-168-78-24.us-west-2.compute.internal",
              "namespace": "",
              "resourceVersion": "744956",
              "selfLink": "/api/v1/nodes/ip-192-168-78-24.us-west-2.compute.internal",
              "uid": "832acc85-cf5f-11e8-b53f-0a31e8f101f8"
          },
          "spec": {
              "externalID": "i-07694ec7d39814736",
              "providerID": "aws:///us-west-2a/i-07694ec7d39814736"
          },
          "status": {
              "addresses": [
                  {
                      "address": "192.168.78.24",
                      "type": "InternalIP"
                  },
                  {
                      "address": "ip-192-168-78-24.us-west-2.compute.internal",
                      "type": "Hostname"
                  }
              ],
              "allocatable": {
                  "cpu": "2",
                  "ephemeral-storage": "48307038948",
                  "hugepages-1Gi": "0",
                  "hugepages-2Mi": "0",
                  "memory": "7765052Ki",
                  "pods": "29"
              },
              "capacity": {
                  "cpu": "2",
                  "ephemeral-storage": "52416492Ki",
                  "hugepages-1Gi": "0",
                  "hugepages-2Mi": "0",
                  "memory": "7867452Ki",
                  "pods": "29"
              },
              "conditions": [
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:16Z",
                      "message": "kubelet has sufficient disk space available",
                      "reason": "KubeletHasSufficientDisk",
                      "status": "False",
                      "type": "OutOfDisk"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:16Z",
                      "message": "kubelet has sufficient memory available",
                      "reason": "KubeletHasSufficientMemory",
                      "status": "False",
                      "type": "MemoryPressure"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:16Z",
                      "message": "kubelet has no disk pressure",
                      "reason": "KubeletHasNoDiskPressure",
                      "status": "False",
                      "type": "DiskPressure"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:16Z",
                      "message": "kubelet has sufficient PID available",
                      "reason": "KubeletHasSufficientPID",
                      "status": "False",
                      "type": "PIDPressure"
                  },
                  {
                      "lastHeartbeatTime": "2018-10-19T17:48:32Z",
                      "lastTransitionTime": "2018-10-14T03:16:36Z",
                      "message": "kubelet is posting ready status",
                      "reason": "KubeletReady",
                      "status": "True",
                      "type": "Ready"
                  }
              ],
              "daemonEndpoints": {
                  "kubeletEndpoint": {
                      "Port": 10250
                  }
              },
              "nodeInfo": {
                  "architecture": "amd64",
                  "bootID": "349fedfb-2861-4d1e-ad15-f2d435389c8f",
                  "containerRuntimeVersion": "docker://17.6.2",
                  "kernelVersion": "4.14.62-70.117.amzn2.x86_64",
                  "kubeProxyVersion": "v1.10.3",
                  "kubeletVersion": "v1.10.3",
                  "machineID": "ec2471d7f414ca97c8eb56b1947cc2c2",
                  "operatingSystem": "linux",
                  "osImage": "Amazon Linux 2",
                  "systemUUID": "EC2471D7-F414-CA97-C8EB-56B1947CC2C2"
              }
          }
      }
  ],
  "kind": "List",
  "metadata": {
      "resourceVersion": "",
      "selfLink": ""
  }
}
`
