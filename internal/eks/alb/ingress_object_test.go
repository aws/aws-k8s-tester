package alb

import "testing"

func Test_getHostnameFromKubectlGetIngressOutput(t *testing.T) {
	h := getHostnameFromKubectlGetIngressOutput([]byte(sampleKubectlGetIngressOutput1), "alb-ingress-controller-service")
	if h != "431f09fb-kubesystem-ingres-1d73-626628990.us-west-2.elb.amazonaws.com" {
		t.Fatalf("unexpected host name %q", h)
	}

	h = getHostnameFromKubectlGetIngressOutput([]byte(sampleKubectlGetIngressOutput2), "alb-ingress-controller-service")
	if h != "fb1dd3ab-kubesystem-ingres-6aec-737236003.us-west-2.elb.amazonaws.com" {
		t.Fatalf("unexpected host name %q", h)
	}
}

const sampleKubectlGetIngressOutput1 = `
apiVersion: v1
items:
- apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    annotations:
      alb.ingress.kubernetes.io/listen-ports: '[{"HTTP":80,"HTTPS": 443}]'
      alb.ingress.kubernetes.io/scheme: internet-facing
      alb.ingress.kubernetes.io/security-groups: sg-030ef05c31a440887,sg-0bf04096324d3851c
      alb.ingress.kubernetes.io/subnets: subnet-042c0447004113fe2,subnet-088d16fbc818d2b31,subnet-01d9fed2fa91bdeef
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"extensions/v1beta1","kind":"Ingress","metadata":{"annotations":{"alb.ingress.kubernetes.io/listen-ports":"[{\"HTTP\":80,\"HTTPS\": 443}]","alb.ingress.kubernetes.io/scheme":"internet-facing","alb.ingress.kubernetes.io/security-groups":"sg-030ef05c31a440887,sg-0bf04096324d3851c","alb.ingress.kubernetes.io/subnets":"subnet-042c0447004113fe2,subnet-088d16fbc818d2b31,subnet-01d9fed2fa91bdeef"},"labels":{"app":"ingress-for-alb"},"name":"ingress-for-alb","namespace":"kube-system"},"spec":{"rules":[{"http":{"paths":[{"backend":{"serviceName":"alb-ingress-controller-service","servicePort":80},"path":"/metrics"}]}}]}}
    creationTimestamp: 2018-09-16T22:07:55Z
    generation: 1
    labels:
      app: ingress-for-alb
    name: ingress-for-alb
    namespace: kube-system
    resourceVersion: "2683"
    selfLink: /apis/extensions/v1beta1/namespaces/kube-system/ingresses/ingress-for-alb
    uid: f69060e7-b9fc-11e8-852e-02d628d39cc2
  spec:
    rules:
    - http:
        paths:
        - backend:
            serviceName: alb-ingress-controller-service
            servicePort: 80
          path: /metrics
  status:
    loadBalancer:
      ingress:
      - hostname: 431f09fb-kubesystem-ingres-1d73-626628990.us-west-2.elb.amazonaws.com
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""

`

const sampleKubectlGetIngressOutput2 = `
apiVersion: v1
items:
- apiVersion: extensions/v1beta1
  kind: Ingress
  metadata:
    annotations:
      alb.ingress.kubernetes.io/listen-ports: '[{"HTTP":80,"HTTPS": 443}]'
      alb.ingress.kubernetes.io/scheme: internet-facing
      alb.ingress.kubernetes.io/security-groups: sg-023a0ffcb1edaafda,sg-0b5b47e3ad4a52b7f
      alb.ingress.kubernetes.io/subnets: subnet-0c23701723aa400a4,subnet-0cd6ca60361995069,subnet-0fbaeb0877273db3a
      kubectl.kubernetes.io/last-applied-configuration: |
        {"apiVersion":"extensions/v1beta1","kind":"Ingress","metadata":{"annotations":{"alb.ingress.kubernetes.io/listen-ports":"[{\"HTTP\":80,\"HTTPS\": 443}]","alb.ingress.kubernetes.io/scheme":"internet-facing","alb.ingress.kubernetes.io/security-groups":"sg-023a0ffcb1edaafda,sg-0b5b47e3ad4a52b7f","alb.ingress.kubernetes.io/subnets":"subnet-0c23701723aa400a4,subnet-0cd6ca60361995069,subnet-0fbaeb0877273db3a"},"creationTimestamp":null,"labels":{"app":"ingress-for-alb-ingress-controller-service"},"name":"ingress-for-alb-ingress-controller-service","namespace":"kube-system"},"spec":{"rules":[{"http":{"paths":[{"backend":{"serviceName":"alb-ingress-controller-service","servicePort":80},"path":"/metrics"}]}}]},"status":{"loadBalancer":{}}}
    creationTimestamp: 2018-09-17T12:49:25Z
    generation: 1
    labels:
      app: ingress-for-alb-ingress-controller-service
    name: ingress-for-alb-ingress-controller-service
    namespace: kube-system
    resourceVersion: "1162"
    selfLink: /apis/extensions/v1beta1/namespaces/kube-system/ingresses/ingress-for-alb-ingress-controller-service
    uid: 1b528955-ba78-11e8-9a72-0a8f21cb6804
  spec:
    rules:
    - http:
        paths:
        - backend:
            serviceName: alb-ingress-controller-service
            servicePort: 80
          path: /metrics
  status:
    loadBalancer:
      ingress:
      - hostname: fb1dd3ab-kubesystem-ingres-6aec-737236003.us-west-2.elb.amazonaws.com
kind: List
metadata:
  resourceVersion: ""
  selfLink: ""

`
