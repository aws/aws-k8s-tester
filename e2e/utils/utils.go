package utils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/aws/aws-k8s-tester/e2e/framework"
	"github.com/aws/aws-k8s-tester/e2e/resources"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/cihub/seelog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetExpectedNodeENIs takes in the node's internal IP and waits until the expected number of ENIs are ready and returns them
func GetExpectedNodeENIs(ctx context.Context, f *framework.Framework, internalIP string, expectedENICount int) []*ec2.NetworkInterface {
	// Get instance IDs
	filterName := "private-ip-address"
	describeInstancesInput := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   &filterName,
				Values: []*string{&internalIP},
			},
		},
	}
	// Get instances from their instance IDs
	instances, err := f.Cloud.EC2().DescribeInstancesAsList(aws.BackgroundContext(), describeInstancesInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				log.Debug(aerr)
			}
		} else {
			log.Debug(err)
		}
	}
	Expect(err).ShouldNot(HaveOccurred())
	Expect(len(instances)).To(Equal(1))

	By("checking number of ENIs via EC2")
	// Wait for ENIs to be ready
	filterName = "attachment.instance-id"
	describeNetworkInterfacesInput := &ec2.DescribeNetworkInterfacesInput{
		Filters: []*ec2.Filter{
			{
				Name:   &filterName,
				Values: []*string{instances[0].InstanceId},
			},
		},
	}

	By(fmt.Sprintf("waiting for number of ENIs via EC2 to equal %d", expectedENICount))
	werr := f.Cloud.EC2().WaitForDesiredNetworkInterfaceCount(describeNetworkInterfacesInput, expectedENICount)
	if werr != nil {
		if aerr, ok := werr.(awserr.Error); ok {
			switch aerr.Code() {
			case request.WaiterResourceNotReadyErrorCode:
				log.Debug(request.WaiterResourceNotReadyErrorCode, aerr.Error())
			default:
				log.Debug(aerr)
			}
		} else {
			log.Debug(err)
		}
	}

	describeNetworkInterfacesOutput, err := f.Cloud.EC2().DescribeNetworkInterfaces(describeNetworkInterfacesInput)
	Expect(err).ShouldNot(HaveOccurred())
	if werr != nil {
		log.Debugf("")
	}

	return describeNetworkInterfacesOutput.NetworkInterfaces
}

// findEnvVar looks for a environment variable name in the container's list of EnvVars
func findEnvVar(envVars []corev1.EnvVar, name string) int {
	for i, envVar := range envVars {
		if name == envVar.Name {
			return i
		}
	}
	return -1
}

// updateDaemonSetEnvVars adds or replaces EnvVar Values
func updateDaemonSetEnvVars(ds *appsv1.DaemonSet, envs []corev1.EnvVar) {
	for c, container := range ds.Spec.Template.Spec.Containers {
		log.Debugf("add/replace env vars for daemonset (%s) container (%s)", ds.Name, container.Name)
		for _, env := range envs {
			i := findEnvVar(container.Env, env.Name)
			if i == -1 {
				log.Debugf("add env var (name: '%s' value: '%s')", env.Name, env.Value)
				ds.Spec.Template.Spec.Containers[c].Env = append(ds.Spec.Template.Spec.Containers[c].Env, env)
			} else if container.Env[i].Value != env.Value {
				log.Debugf("replace env var (name: '%s' value: '%s' -> '%s')", env.Name, container.Env[i].Value, env.Value)
				ds.Spec.Template.Spec.Containers[c].Env[i].Value = env.Value
			} else {
				log.Debugf("no change for env var (name: '%s' value: '%s')", env.Name, container.Env[i].Value)
			}
		}
	}
}

// UpdateDaemonSetEnvVars updates a daemonset with updated environment variables
func UpdateDaemonSetEnvVars(ctx context.Context, f *framework.Framework, ns *corev1.Namespace, ds *appsv1.DaemonSet, envs []corev1.EnvVar) {
	var err error
	ds, err = f.ClientSet.AppsV1().DaemonSets(ns.Name).Get(ds.Name, metav1.GetOptions{})
	Expect(err).ShouldNot(HaveOccurred())

	updateDaemonSetEnvVars(ds, envs)

	// Update daemonset
	resource := &resources.Resources{
		Daemonset: ds,
	}
	resource.ExpectDaemonsetUpdateSuccessful(ctx, f, ns)
}

// UpdateDaemonSetLabels updates labels for a daemonset
func UpdateDaemonSetLabels(ctx context.Context, f *framework.Framework, ns *corev1.Namespace, ds *appsv1.DaemonSet, labels map[string]string) {
	var err error
	ds, err = f.ClientSet.AppsV1().DaemonSets(ns.Name).Get(ds.Name, metav1.GetOptions{})
	Expect(err).ShouldNot(HaveOccurred())

	for k, v := range labels {
		ds.Spec.Template.Labels[k] = v
	}

	// Update daemonset
	resource := &resources.Resources{
		Daemonset: ds,
	}
	resource.ExpectDaemonsetUpdateSuccessful(ctx, f, ns)
}

// GetTesterPodNodeName gets the node name in which the pod runs on
func GetTesterPodNodeName(f *framework.Framework, nsName string, podName string) (string, error) {
	testerPod, err := f.ClientSet.CoreV1().Pods(nsName).Get(podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return testerPod.Spec.NodeName, err
}

// GetTestNodes gets the nodes that are not running the tests
// TODO handle node status
func GetTestNodes(f *framework.Framework, testerNodeName string) ([]corev1.Node, error) {
	var testNodes []corev1.Node

	nodesList, err := f.ClientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(nodesList.Items) == 0 {
		return nil, errors.New("No nodes found")
	}

	for _, node := range nodesList.Items {
		if testerNodeName != node.Name {
			log.Debugf("Found test node (%s)", node.Name)
			testNodes = append(testNodes, node)
		}
	}
	return testNodes, nil
}

// NodeCoreDNSCount returns the number of KubeDNS or CoreDNS pods running on the node
func NodeCoreDNSCount(f *framework.Framework, nodeName string) (int, error) {
	var err error
	var count int
	var podList *corev1.PodList

	// Find nodes running coredns
	listOptions := metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	}
	podList, err = f.ClientSet.CoreV1().Pods("kube-system").List(listOptions)
	if err != nil {
		return 0, err
	}
	for _, pod := range podList.Items {
		if nodeName == pod.Spec.NodeName {
			count++
		}
	}
	if count != 0 {
		log.Debugf("found (%d) CoreDNS pods running on node (%s)", count, nodeName)
	}
	return count, nil
}

// GetNodeInternalIP gets a node's internal IP address
func GetNodeInternalIP(node corev1.Node) (string, error) {
	if len(node.Status.Addresses) == 0 {
		return "", fmt.Errorf("No addresses found for node (%s)", node.Name)
	}

	var internalIP string
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			internalIP = address.Address
		}
	}
	return internalIP, nil
}

// WaitForASGDesiredCapacity ensures the autoscaling groups have the requested desired capacity and the nodes become ready
// This requires an IAM policy with EC2 autoscaling:UpdateAutoScalingGroup to update the max size
func WaitForASGDesiredCapacity(ctx context.Context, f *framework.Framework, nodes []corev1.Node, desiredCapacity int) error {
	var asgNames []*string
	var asgs []*autoscaling.Group
	var nodeNames []*string
	var instanceIDs []*string

	if len(nodes) >= desiredCapacity {
		return nil
	}

	for _, node := range nodes {
		nodeName := node.Name
		nodeNames = append(nodeNames, &nodeName)
	}

	// Get instance IDs
	filterName := "private-dns-name"
	describeInstancesInput := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   &filterName,
				Values: nodeNames,
			},
		},
	}
	instances, err := f.Cloud.EC2().DescribeInstancesAsList(aws.BackgroundContext(), describeInstancesInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				log.Debug(aerr)
			}
		} else {
			log.Debug(err)
		}
		return err
	}
	if len(instances) == 0 {
		return errors.New("No instances found")
	}

	// Get ASGs
	for _, instance := range instances {
		instanceIDs = append(instanceIDs, instance.InstanceId)
	}
	describeAutoScalingInstancesInput := &autoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: instanceIDs,
	}
	asgInstanceDetails, err := f.Cloud.AutoScaling().DescribeAutoScalingInstancesAsList(aws.BackgroundContext(), describeAutoScalingInstancesInput)
	if err != nil {
		return err
	}
	for _, asgInstance := range asgInstanceDetails {
		asgNames = append(asgNames, asgInstance.AutoScalingGroupName)
	}
	describeASGsInput := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: asgNames,
	}
	asgs, err = f.Cloud.AutoScaling().DescribeAutoScalingGroupsAsList(aws.BackgroundContext(), describeASGsInput)
	if err != nil {
		return err
	}

	log.Debug("verifying desired capacity of ASGs")
	cap := int64(math.Ceil(float64(desiredCapacity) / float64(len(asgs))))
	for _, asg := range asgs {
		if *(asg.DesiredCapacity) < cap {
			log.Debugf("")
			max := *(asg.MaxSize)
			if max < cap {
				max = cap
			}
			log.Debugf("increasing ASG desired capacity to %d", cap)
			updateAutoScalingGroupInput := &autoscaling.UpdateAutoScalingGroupInput{
				AutoScalingGroupName: asg.AutoScalingGroupName,
				DesiredCapacity:      &cap,
				MaxSize:              &max,
			}
			_, err := f.Cloud.AutoScaling().UpdateAutoScalingGroup(updateAutoScalingGroupInput)
			if err != nil {
				return err
			}
		}
	}

	// Wait for instances and nodes to be ready
	return WaitForASGInstancesAndNodesReady(ctx, f, describeASGsInput)
}

// ReplaceNodeASGInstances terminates instances for given nodes, waits for new instances to be
// ready in their autoscaling groups, and waits for the new nodes to be ready
// This requires an IAM policy with EC2 autoscaling:TerminateInstanceInAutoScalingGroup
func ReplaceNodeASGInstances(ctx context.Context, f *framework.Framework, nodes []corev1.Node) error {
	var asgs []*string
	var nodeNames []*string
	var instanceIDsTerminate []*string

	for _, node := range nodes {
		nodeName := node.Name
		nodeNames = append(nodeNames, &nodeName)
	}

	// Get instance IDs
	filterName := "private-dns-name"
	describeInstancesInput := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   &filterName,
				Values: nodeNames,
			},
		},
	}
	instancesToTerminate, err := f.Cloud.EC2().DescribeInstancesAsList(aws.BackgroundContext(), describeInstancesInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				log.Debug(aerr)
			}
		} else {
			log.Debug(err)
		}
		return err
	}
	if len(instancesToTerminate) == 0 {
		return errors.New("No instances found")
	}
	for i, instance := range instancesToTerminate {
		log.Debugf("Terminating instance %d/%d (name: %v, id: %v)", i+1, len(instancesToTerminate), *(instance.PrivateDnsName), *(instance.InstanceId))
		instanceIDsTerminate = append(instanceIDsTerminate, instance.InstanceId)
	}
	// Terminate instances
	for _, instanceID := range instanceIDsTerminate {
		terminateInstanceInASGInput := &autoscaling.TerminateInstanceInAutoScalingGroupInput{
			InstanceId:                     aws.String(*instanceID),
			ShouldDecrementDesiredCapacity: aws.Bool(false),
		}
		result, err := f.Cloud.AutoScaling().TerminateInstanceInAutoScalingGroup(terminateInstanceInASGInput)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case autoscaling.ErrCodeScalingActivityInProgressFault:
					log.Debug(autoscaling.ErrCodeScalingActivityInProgressFault, aerr.Error())
				case autoscaling.ErrCodeResourceContentionFault:
					log.Debug(autoscaling.ErrCodeResourceContentionFault, aerr.Error())
				default:
					log.Debug(aerr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				log.Debug(err.Error())
			}
			return err
		}
		asgs = append(asgs, result.Activity.AutoScalingGroupName)
	}
	// Ensure node is in terminating state
	time.Sleep(time.Second * 2)

	// Wait for ASGs to be in service
	describeASGsInput := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: asgs,
	}

	// Wait for instances and nodes to be ready
	return WaitForASGInstancesAndNodesReady(ctx, f, describeASGsInput)
}

// WaitForASGInstancesAndNodesReady waits for ASG instances and nodes to be ready
func WaitForASGInstancesAndNodesReady(ctx context.Context, f *framework.Framework, describeASGsInput *autoscaling.DescribeAutoScalingGroupsInput) error {
	var asgInstanceIDs []*string

	By("wait until ASG instances are ready")
	err := f.Cloud.AutoScaling().WaitUntilAutoScalingGroupInService(aws.BackgroundContext(), describeASGsInput)

	// Get instance IDs
	asgInstances, err := f.Cloud.AutoScaling().DescribeInServiceAutoScalingGroupInstancesAsList(aws.BackgroundContext(), describeASGsInput)
	if err != nil {
		return err
	}

	By("wait nodes ready")
	for i, asgInstance := range asgInstances {
		log.Debugf("Instance %d/%d (id: %s) is in service", i+1, len(asgInstances), *(asgInstance.InstanceId))
		asgInstanceIDs = append(asgInstanceIDs, asgInstance.InstanceId)
	}
	describeInstancesInput := &ec2.DescribeInstancesInput{
		InstanceIds: asgInstanceIDs,
	}
	instancesList, err := f.Cloud.EC2().DescribeInstancesAsList(aws.BackgroundContext(), describeInstancesInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				log.Debug(aerr)
			}
		} else {
			log.Debug(err)
		}
		return err
	}

	// Wait until nodes exist and are ready
	for i, instance := range instancesList {
		nodeName := instance.PrivateDnsName
		log.Debugf("Wait until node %d/%d (%s) exists", i+1, len(instancesList), *nodeName)
		node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: *nodeName}}
		node, err = f.ResourceManager.WaitNodeExists(ctx, node)
		if err != nil {
			return err
		}
		log.Debugf("Wait until node %d/%d (%s) ready", i+1, len(instancesList), *nodeName)
		_, err = f.ResourceManager.WaitNodeReady(ctx, node)
		if err != nil {
			return err
		}
	}
	return nil
}

// AddAnnotationsToDaemonSet adds annotations to a daemonset
func AddAnnotationsToDaemonSet(ctx context.Context, f *framework.Framework, ns *corev1.Namespace, ds *appsv1.DaemonSet, annotations map[string]string) error {
	ds, err := f.ClientSet.AppsV1().DaemonSets(ns.Name).Get(ds.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if ds.Spec.Template.ObjectMeta.Annotations == nil {
		ds.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
	}

	for k, v := range annotations {
		if val, ok := ds.Spec.Template.ObjectMeta.Annotations[k]; !ok {
			log.Debugf("Adding annotation (%s: '%s') to daemonset (%s)", k, v, ds.Name)
		} else if v != val {
			log.Debugf("Replacing annotation (%s: '%s' -> '%s') on daemonset (%s)", k, v, val, ds.Name)
		}
		ds.Spec.Template.ObjectMeta.Annotations[k] = v
	}

	resource := &resources.Resources{
		Daemonset: ds,
	}

	resource.ExpectDaemonsetUpdateSuccessful(ctx, f, ns)
	return nil
}
