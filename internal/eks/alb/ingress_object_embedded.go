package alb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/awstester/internal/eks/alb/ingress"
	"github.com/aws/awstester/internal/eks/alb/ingress/path"
	"github.com/aws/awstester/pkg/httputil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	humanize "github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// TODO: split into separate functions...

func (md *embedded) CreateIngressObjects() error {
	if md.cfg.ClusterState.CFStackVPCID == "" {
		return errors.New("cannot create Ingress object without VPC stack VPC ID")
	}
	if md.cfg.ClusterState.CFStackVPCSecurityGroupID == "" {
		return errors.New("cannot create Ingress object without VPC stack Security Group ID")
	}
	if len(md.cfg.ClusterState.CFStackVPCSubnetIDs) == 0 {
		return errors.New("cannot create Ingress object without VPC stack Subnet IDs")
	}
	if md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen == "" {
		return errors.New("cannot create Ingress object without ALB Ingress Controller Security Group ID")
	}

	md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "CREATING"
	md.cfg.ALBIngressController.IngressRuleStatusDefault = "CREATING"
	md.cfg.Sync()

	now := time.Now().UTC()

	h, _ := os.Hostname()
	cfg1 := ingress.ConfigIngressTestServerIngressSpec{
		MetadataName:      "ingress-for-alb-ingress-controller-service",
		MetadataNamespace: "kube-system",
		Tags: map[string]string{
			md.cfg.Tag: md.cfg.ClusterName,
			"HOSTNAME": h,
		},
		TargetType: md.cfg.ALBIngressController.TargetType,
		SubnetIDs:  md.cfg.ClusterState.CFStackVPCSubnetIDs,

		// security group associated with an instance
		// must allow traffic from the load balancer
		// populate this only when the target type is "instance"
		// pod "ip" model should let ingress controller create a new security group
		SecurityGroupIDs: nil,

		IngressPaths: []v1beta1.HTTPIngressPath{
			{
				Path: "/metrics",
				Backend: v1beta1.IngressBackend{
					ServiceName: "alb-ingress-controller-service",
					ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(80)},
				},
			},
		},
	}
	if md.cfg.ALBIngressController.TargetType == "instance" {
		cfg1.SecurityGroupIDs = []string{
			md.cfg.ClusterState.CFStackVPCSecurityGroupID,
			md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen,
		}
	}
	if md.cfg.LogAccess {
		cfg1.LogAccess = fmt.Sprintf(
			"access_logs.s3.enabled=true,access_logs.s3.bucket=%s,access_logs.s3.prefix=%s-kube-system",
			md.s3Plugin.BucketForAccessLogs(),
			md.cfg.ClusterName,
		)
	}

	d1, err := ingress.CreateIngressTestServerIngressSpec(cfg1)
	if err != nil {
		md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = err.Error()
		md.cfg.ALBIngressController.IngressRuleStatusDefault = err.Error()
		md.cfg.Sync()
		return err
	}

	cfg2 := ingress.ConfigIngressTestServerIngressSpec{
		MetadataName:      "ingress-for-ingress-test-server-service",
		MetadataNamespace: "default",
		Tags: map[string]string{
			md.cfg.Tag: md.cfg.ClusterName,
			"HOSTNAME": h,
		},
		TargetType: md.cfg.ALBIngressController.TargetType,
		SubnetIDs:  md.cfg.ClusterState.CFStackVPCSubnetIDs,
		SecurityGroupIDs: []string{
			md.cfg.ClusterState.CFStackVPCSecurityGroupID,
			md.cfg.ALBIngressController.ELBv2SecurityGroupIDPortOpen,
		},
		GenTargetServicePort: 80,
	}
	switch md.cfg.ALBIngressController.TestMode {
	case "ingress-test-server":
		cfg2.IngressPaths = []v1beta1.HTTPIngressPath{
			{
				Path: path.Path,
				Backend: v1beta1.IngressBackend{
					ServiceName: "ingress-test-server-service",
					ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(80)},
				},
			},
			{
				Path: path.PathMetrics,
				Backend: v1beta1.IngressBackend{
					ServiceName: "ingress-test-server-service",
					ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(80)},
				},
			},
		}
		cfg2.GenTargetServiceName = "ingress-test-server-service"
		cfg2.GenTargetServiceRoutesN = md.cfg.ALBIngressController.TestServerRoutes

	case "nginx":
		cfg2.IngressPaths = []v1beta1.HTTPIngressPath{
			{
				Path: "/*",
				Backend: v1beta1.IngressBackend{
					ServiceName: "nginx-service",
					ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: int32(80)},
				},
			},
		}
		cfg2.GenTargetServiceName = "nginx-service"
		cfg2.GenTargetServiceRoutesN = 0
	}

	if md.cfg.LogAccess {
		cfg2.LogAccess = fmt.Sprintf(
			"access_logs.s3.enabled=true,access_logs.s3.bucket=%s,access_logs.s3.prefix=%s-default",
			md.s3Plugin.BucketForAccessLogs(),
			md.cfg.ClusterName,
		)
	}

	d2, err := ingress.CreateIngressTestServerIngressSpec(cfg2)
	if err != nil {
		md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = err.Error()
		md.cfg.ALBIngressController.IngressRuleStatusDefault = err.Error()
		md.cfg.Sync()
		return err
	}

	d := fmt.Sprintf(`---
%s



---
%s



`, d1, d2)

	f, err := os.OpenFile(md.cfg.ALBIngressController.IngressObjectSpecPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = err.Error()
		md.cfg.ALBIngressController.IngressRuleStatusDefault = err.Error()
		md.cfg.Sync()
		return err
	}

	_, err = f.Write([]byte(d))
	if err != nil {
		md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = err.Error()
		md.cfg.ALBIngressController.IngressRuleStatusDefault = err.Error()
		md.cfg.Sync()
		return err
	}
	f.Close()

	var kexo []byte
	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"apply",
			"--filename="+md.cfg.ALBIngressController.IngressObjectSpecPath,
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to apply ingress object",
				zap.String("output", string(kexo)),
				zap.Error(err),
			)
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = err.Error()
			md.cfg.Sync()
			time.Sleep(10 * time.Second)
			continue
		}
		md.lg.Info("applied ingress object", zap.String("output", string(kexo)))
		md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "APPLIED"
		md.cfg.Sync()
		break
	}

	md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "CREATING"
	md.cfg.Sync()

	// usually takes 2-minute
	md.lg.Info("waiting for 2-minute")
	time.Sleep(2 * time.Minute)

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "ingress",
			"--namespace=kube-system",
			"--output=yaml",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get ingress", zap.String("namespace", "kube-system"), zap.Error(err))
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = err.Error()
			md.cfg.Sync()
			time.Sleep(15 * time.Second)
			continue
		}

		h := getHostnameFromKubectlGetIngressOutput(kexo, cfg1.IngressPaths[0].Backend.ServiceName)
		if h != "*" {
			md.lg.Info("created ingress",
				zap.String("service-name", cfg1.IngressPaths[0].Backend.ServiceName),
				zap.String("namespace", "kube-system"),
				zap.String("host", h),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)
			if len(md.cfg.ALBIngressController.ELBv2NamespaceToDNSName) == 0 {
				md.cfg.ALBIngressController.ELBv2NamespaceToDNSName = make(map[string]string)
			}
			if len(md.cfg.ALBIngressController.ELBv2NameToDNSName) == 0 {
				md.cfg.ALBIngressController.ELBv2NameToDNSName = make(map[string]string)
			}
			md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["kube-system"] = h
			md.cfg.ALBIngressController.ELBv2NameToDNSName[strings.Join(strings.Split(h, "-")[:4], "-")] = h
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "READY"
			md.cfg.Sync()
			break
		}

		md.lg.Info("creating ingress",
			zap.String("output", string(kexo)),
			zap.String("namespace", "kube-system"),
			zap.String("host", h),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "CREATING"
		md.cfg.Sync()
		time.Sleep(10 * time.Second)
		continue
	}

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "ingress",
			"--namespace=default",
			"--output=yaml",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err != nil {
			md.lg.Warn("failed to get ingress", zap.String("namespace", "default"), zap.Error(err))
			md.cfg.ALBIngressController.IngressRuleStatusDefault = err.Error()
			md.cfg.Sync()
			time.Sleep(15 * time.Second)
			continue
		}

		h := getHostnameFromKubectlGetIngressOutput(kexo, cfg2.IngressPaths[0].Backend.ServiceName)
		if h != "*" {
			md.lg.Info("created ingress",
				zap.String("service-name", cfg2.IngressPaths[0].Backend.ServiceName),
				zap.String("namespace", "default"),
				zap.String("host", h),
				zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			)
			if len(md.cfg.ALBIngressController.ELBv2NamespaceToDNSName) == 0 {
				md.cfg.ALBIngressController.ELBv2NamespaceToDNSName = make(map[string]string)
			}
			if len(md.cfg.ALBIngressController.ELBv2NameToDNSName) == 0 {
				md.cfg.ALBIngressController.ELBv2NameToDNSName = make(map[string]string)
			}
			md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"] = h
			md.cfg.ALBIngressController.ELBv2NameToDNSName[strings.Join(strings.Split(h, "-")[:4], "-")] = h
			md.cfg.ALBIngressController.IngressRuleStatusDefault = "READY"
			md.cfg.Sync()
			break
		}

		md.lg.Info("creating ingress",
			zap.String("output", string(kexo)),
			zap.String("namespace", "default"),
			zap.String("host", h),
			zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
			zap.Error(err),
		)
		md.cfg.ALBIngressController.IngressRuleStatusDefault = "CREATING"
		md.cfg.Sync()
		time.Sleep(10 * time.Second)
		continue
	}
	md.lg.Info("created ingress",
		zap.String("dns-name-kube-system", md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["kube-system"]),
		zap.String("dns-name-default", md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"]),
		zap.String("request-started", humanize.RelTime(now, time.Now().UTC(), "ago", "from now")),
	)

	md.lg.Info("searching for AWS ELBv2 resources",
		zap.String("elbv2-name-to-dns-name", fmt.Sprintf("%v", md.cfg.ALBIngressController.ELBv2NameToDNSName)),
	)
	names := make([]string, 0, len(md.cfg.ALBIngressController.ELBv2NameToDNSName))
	for k := range md.cfg.ALBIngressController.ELBv2NameToDNSName {
		names = append(names, k)
	}
	eo, oerr := md.elbv2.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
		Names: aws.StringSlice(names),
	})
	if oerr != nil {
		md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = oerr.Error()
		md.cfg.ALBIngressController.IngressRuleStatusDefault = oerr.Error()
		md.cfg.Sync()
		return oerr
	}
	for _, lb := range eo.LoadBalancers {
		name := *lb.LoadBalancerName
		h, ok := md.cfg.ALBIngressController.ELBv2NameToDNSName[name]
		if !ok {
			ev := fmt.Errorf("ELBv2 name %q not found on AWS", name)
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = ev.Error()
			md.cfg.ALBIngressController.IngressRuleStatusDefault = ev.Error()
			md.cfg.Sync()
			return ev
		}
		if h != *lb.DNSName {
			ev := fmt.Errorf("ELBv2 name %q has different DNS name %q != %q", name, h, *lb.DNSName)
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = ev.Error()
			md.cfg.ALBIngressController.IngressRuleStatusDefault = ev.Error()
			md.cfg.Sync()
			return ev
		}
		if len(md.cfg.ALBIngressController.ELBv2NameToARN) == 0 {
			md.cfg.ALBIngressController.ELBv2NameToARN = make(map[string]string)
		}
		md.cfg.ALBIngressController.ELBv2NameToARN[name] = *lb.LoadBalancerArn
	}

	if len(md.cfg.ALBIngressController.ELBv2NamespaceToDNSName) != 2 {
		return fmt.Errorf("expected two ELBv2 DNS names, got %+v", md.cfg.ALBIngressController.ELBv2NamespaceToDNSName)
	}

	md.lg.Info("found AWS ELBv2 resources",
		zap.String("elbv2-namespace-to-dns-name", fmt.Sprintf("%v", md.cfg.ALBIngressController.ELBv2NamespaceToDNSName)),
		zap.String("elbv2-name-to-dns-name", fmt.Sprintf("%v", md.cfg.ALBIngressController.ELBv2NameToDNSName)),
		zap.String("elbv2-name-to-arn", fmt.Sprintf("%v", md.cfg.ALBIngressController.ELBv2NameToARN)),
	)

	ep := "http://" + md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"]
	if md.cfg.ALBIngressController.TestMode == "ingress-test-server" {
		ep += path.Path
	}
	if !httputil.CheckGet(
		md.lg,
		ep,
		strings.Repeat("0", md.cfg.ALBIngressController.TestResponseSize),
		30,
		10*time.Second,
		md.stopc,
	) {
		return errors.New("ingress for 'default' is not ready")
	}
	md.lg.Info("created ingress", zap.String("namespace", "default"))

	if !httputil.CheckGet(
		md.lg,
		"http://"+md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["kube-system"]+"/metrics",
		"",
		30,
		10*time.Second,
		md.stopc,
	) {
		return errors.New("Ingress for 'kube-system' is not ready")
	}
	md.lg.Info("created ingress", zap.String("namespace", "kube-system"))

	return md.cfg.Sync()
}

// DeleteIngressObjects deletes ingress objects.
// cloudformation delete often fails due to ELBv2 dependencies.
// Delete ELBv2 first and see if that helps.
func (md *embedded) DeleteIngressObjects() error {
	if len(md.cfg.ALBIngressController.ELBv2NamespaceToDNSName) == 0 {
		return errors.New("cannot find any ELBv2 DNS names (previous step might have failed)")
	}
	if len(md.cfg.ALBIngressController.ELBv2NameToARN) == 0 {
		return errors.New("cannot find ELBv2 ARNs to delete (previous step might have failed)")
	}

	md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "DELETING"
	md.cfg.ALBIngressController.IngressRuleStatusDefault = "DELETING"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := md.kubectl.CommandContext(ctx,
		md.kubectlPath,
		"--kubeconfig="+md.cfg.KubeConfigPath,
		"delete",
		"--filename="+md.cfg.ALBIngressController.IngressObjectSpecPath,
	)
	kexo, err := cmd.CombinedOutput()
	cancel()

	retryStart := time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		cmd = md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "ingress",
			"-o=wide",
			"--namespace=kube-system",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()

		addr := md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["kube-system"]

		if err == nil {
			// assume we only deploy 1 ingress per namespace
			if !strings.Contains(string(kexo), addr) ||
				strings.Contains(string(kexo), "No resources found.") {
				md.lg.Info("deleted ingress",
					zap.String("namespace", "kube-system"),
					zap.String("dns-name", addr),
					zap.String("output", string(kexo)),
				)
				md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "DELETING (DELETED kube-system Ingress)"
				md.cfg.Sync()
				break
			}
		}

		if strings.Contains(string(kexo), "no such host") {
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "DELETING (DELETED kube-system Ingress)"
			md.cfg.Sync()
			break
		}

		md.lg.Info("deleting ingress",
			zap.String("namespace", "kube-system"),
			zap.String("dns-name", addr),
			zap.String("output", string(kexo)),
			zap.Error(err),
		)
		time.Sleep(5 * time.Second)
	}

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		cmd = md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "ingress",
			"-o=wide",
			"--namespace=default",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()

		addr := md.cfg.ALBIngressController.ELBv2NamespaceToDNSName["default"]

		if err == nil {
			// assume we only deploy 1 ingress per namespace
			if !strings.Contains(string(kexo), addr) ||
				strings.Contains(string(kexo), "No resources found.") {
				md.lg.Info("deleted ingress",
					zap.String("namespace", "default"),
					zap.String("dns-name", addr),
					zap.String("output", string(kexo)),
				)
				md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "DELETED Ingress objects in all namespace"
				md.cfg.ALBIngressController.IngressRuleStatusDefault = "DELETED Ingress objects in all namespace"
				md.cfg.Sync()
				break
			}
		}

		if strings.Contains(string(kexo), "no such host") {
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "DELETED Ingress objects in all namespace"
			md.cfg.ALBIngressController.IngressRuleStatusDefault = "DELETED Ingress objects in all namespace"
			md.cfg.Sync()
			break
		}

		md.lg.Info("deleting ingress",
			zap.String("namespace", "default"),
			zap.String("dns-name", addr),
			zap.String("output", string(kexo)),
			zap.Error(err),
		)
		time.Sleep(5 * time.Second)
	}
	md.lg.Info("deleted ingress")

	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	cmd = md.kubectl.CommandContext(ctx,
		md.kubectlPath,
		"--kubeconfig="+md.cfg.KubeConfigPath,
		"delete", "--filename="+md.cfg.ALBIngressController.IngressControllerSpecPath,
	)
	kexo, err = cmd.CombinedOutput()
	cancel()

	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		cmd = md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "pods",
			"--namespace=kube-system",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err == nil {
			if !strings.Contains(string(kexo), "alb-ingress-controller-") {
				md.lg.Info("deleted alb-ingress-controller deployment", zap.String("namespace", "kube-system"))
				break
			}
		}
		md.lg.Info("deleting alb-ingress-controller deployment",
			zap.String("namespace", "kube-system"),
			zap.Error(err),
		)
		time.Sleep(5 * time.Second)
	}
	retryStart = time.Now().UTC()
	for time.Now().UTC().Sub(retryStart) < 5*time.Minute {
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		cmd = md.kubectl.CommandContext(ctx,
			md.kubectlPath,
			"--kubeconfig="+md.cfg.KubeConfigPath,
			"get", "svc",
			"--namespace=kube-system",
		)
		kexo, err = cmd.CombinedOutput()
		cancel()
		if err == nil {
			if !strings.Contains(string(kexo), "alb-ingress-controller-service") {
				md.lg.Info("deleted alb-ingress-controller-service", zap.String("namespace", "kube-system"))
				break
			}
		}
		md.lg.Info("deleting alb-ingress-controller-service",
			zap.String("namespace", "kube-system"),
			zap.Error(err),
		)
		time.Sleep(5 * time.Second)
	}
	md.lg.Info("deleted ALB Ingress Controller deployment and service")

	time.Sleep(5 * time.Second)

	// garbage collect listeners associated with VPC ID
	// TODO: fix it from upstream, this should be deleted automatic
	// when ELBv2 is deleted
	for name, arn := range md.cfg.ALBIngressController.ELBv2NameToARN {
		var desc *elbv2.DescribeListenersOutput
		desc, err = md.elbv2.DescribeListeners(&elbv2.DescribeListenersInput{
			LoadBalancerArn: aws.String(arn),
		})
		if err != nil {
			md.lg.Warn("failed to describe listener", zap.Error(err))
		} else {
			md.lg.Warn("found listeners", zap.Int("groups", len(desc.Listeners)))
			if len(desc.Listeners) > 0 {
				md.lg.Warn("ALB Ingress Controller garbage collection has not finished!")
			}
			for _, ln := range desc.Listeners {
				_, err = md.elbv2.DeleteListener(&elbv2.DeleteListenerInput{
					ListenerArn: ln.ListenerArn,
				})
				md.lg.Info("deleted listener",
					zap.Int64("listener-port", *ln.Port),
					zap.String("listener-arn", *ln.ListenerArn),
					zap.Error(err),
				)
			}
		}
		md.lg.Info(
			"deleted ELBv2 listener",
			zap.String("vpc-id", md.cfg.ClusterState.CFStackVPCID),
			zap.String("alb-name", name),
			zap.String("alb-arn", arn),
		)
	}

	time.Sleep(5 * time.Second)

	// garbage collect target groups associated with VPC ID
	// TODO: fix it from upstream, this should be deleted automatic
	// when ELBv2 is deleted
	for name, arn := range md.cfg.ALBIngressController.ELBv2NameToARN {
		var desc *elbv2.DescribeTargetGroupsOutput
		desc, err = md.elbv2.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			LoadBalancerArn: aws.String(arn),
		})
		if err != nil {
			md.lg.Debug("failed to describe target group", zap.Error(err))
		} else {
			md.lg.Warn("found target groups", zap.Int("groups", len(desc.TargetGroups)))
			if len(desc.TargetGroups) > 0 {
				md.lg.Warn("ALB Ingress Controller garbage collection has not finished!")
			}
			for _, tg := range desc.TargetGroups {
				_, err = md.elbv2.DeleteTargetGroup(&elbv2.DeleteTargetGroupInput{
					TargetGroupArn: tg.TargetGroupArn,
				})
				md.lg.Info("deleted target group",
					zap.String("vpc-id", *tg.VpcId),
					zap.Int64("port", *tg.Port),
					zap.String("target-type", *tg.TargetType),
					zap.String("target-group-name", *tg.TargetGroupName),
					zap.String("target-group-arn", *tg.TargetGroupArn),
					zap.Error(err),
				)
			}
		}

		desc, err = md.elbv2.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{})
		if err != nil {
			md.lg.Debug("failed to describe target group", zap.Error(err))
		} else {
			md.lg.Info("all target groups", zap.Int("groups", len(desc.TargetGroups)))
			for _, tg := range desc.TargetGroups {
				if *tg.VpcId != md.cfg.ClusterState.CFStackVPCID {
					continue
				}
				_, err = md.elbv2.DeleteTargetGroup(&elbv2.DeleteTargetGroupInput{
					TargetGroupArn: tg.TargetGroupArn,
				})
				md.lg.Info("deleted target group with matching VPC ID",
					zap.String("vpc-id", *tg.VpcId),
					zap.Int64("port", *tg.Port),
					zap.String("target-type", *tg.TargetType),
					zap.String("target-group-name", *tg.TargetGroupName),
					zap.String("target-group-arn", *tg.TargetGroupArn),
					zap.Error(err),
				)
			}
		}
		md.lg.Info(
			"deleted ELBv2 target group",
			zap.String("vpc-id", md.cfg.ClusterState.CFStackVPCID),
			zap.String("alb-name", name),
			zap.String("alb-arn", arn),
		)
	}

	time.Sleep(5 * time.Second)

	// in case ALB Ingress Controller does not clean up ELBv2 resources
	for name, arn := range md.cfg.ALBIngressController.ELBv2NameToARN {
		_, err = md.elbv2.DeleteLoadBalancer(&elbv2.DeleteLoadBalancerInput{
			LoadBalancerArn: aws.String(arn),
		})
		if err != nil {
			// do not fail the whole function, just logging errors
			// ingress object deletion should have cleaned up this resources anyway
			md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = err.Error()
			md.cfg.ALBIngressController.IngressRuleStatusDefault = err.Error()
			md.cfg.Sync()
		}
		md.lg.Info(
			"deleted ELBv2 instance",
			zap.String("alb-name", name),
			zap.String("alb-arn", arn),
			zap.Error(err),
		)
	}

	md.cfg.ALBIngressController.IngressRuleStatusKubeSystem = "DELETE_COMPLETE"
	md.cfg.ALBIngressController.IngressRuleStatusDefault = "DELETE_COMPLETE"
	return md.cfg.Sync()
}
