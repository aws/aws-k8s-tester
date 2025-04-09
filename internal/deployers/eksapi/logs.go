package eksapi

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type logManager struct {
	clients    *awsClients
	resourceID string
}

type deployerPhase string

const (
	deployerPhaseUp   = "up"
	deployerPhaseDown = "down"
)

func NewLogManager(clients *awsClients, resourceID string) *logManager {
	return &logManager{
		clients:    clients,
		resourceID: resourceID,
	}
}

func (m *logManager) gatherLogsFromNodes(k8sClient *k8sClient, opts *deployerOptions, phase deployerPhase) error {
	if opts.LogBucket == "" {
		klog.Info("--log-bucket is empty, no logs will be gathered!")
		return nil
	}
	if k8sClient == nil {
		klog.Infof("no k8s client available, no logs will be gathered!")
		return nil
	}
	if opts.AutoMode {
		return m.gatherLogsUsingNodeDiagnostic(k8sClient, opts, phase)
	}
	switch opts.UserDataFormat {
	case "bootstrap.sh", "nodeadm", "": // if no --user-data-format was passed, we must be using managed nodes, which default to AL-based AMIs
		return m.gatherLogsUsingScript(k8sClient, opts, phase)
	default:
		klog.Warningf("unable to gather logs for userDataFormat: %s\n", opts.UserDataFormat)
		return nil
	}
}

//go:embed logs_ssm_doc.json
var logCollectorScriptSsmDocumentContent string

const logCollectorSsmDocumentTimeout = 5 * time.Minute

func (m *logManager) gatherLogsUsingScript(k8sClient *k8sClient, opts *deployerOptions, phase deployerPhase) error {
	klog.Info("gathering logs using script...")
	nodes, err := k8sClient.clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return err
	}
	var instanceIds []string
	if len(nodes.Items) > 0 {
		instanceIds, err = getNodeInstanceIDs(nodes.Items)
		if err != nil {
			return err
		}
	} else {
		// if we're using unmanaged nodes, we can track down the instances even if they didn't join the cluster
		if opts.UnmanagedNodes {
			paginator := ec2.NewDescribeInstancesPaginator(m.clients.EC2(), &ec2.DescribeInstancesInput{
				Filters: []ec2types.Filter{
					{
						Name:   aws.String("tag:Name"),
						Values: []string{fmt.Sprintf("%s-Node", m.resourceID)},
					},
				},
			})
			for paginator.HasMorePages() {
				out, err := paginator.NextPage(context.TODO())
				if err != nil {
					klog.Warningf("failed to describe unmanaged nodes: %v", err)
					return nil
				}
				for _, reservation := range out.Reservations {
					for _, instance := range reservation.Instances {
						instanceIds = append(instanceIds, *instance.InstanceId)
					}
				}
			}
		}
	}
	if len(instanceIds) == 0 {
		klog.Warning("no nodes to gather logs from!")
		return nil
	}
	doc, err := m.clients.SSM().CreateDocument(context.TODO(), &ssm.CreateDocumentInput{
		Content:        aws.String(logCollectorScriptSsmDocumentContent),
		Name:           aws.String(fmt.Sprintf("%s-log-collector", m.resourceID)),
		DocumentType:   ssmtypes.DocumentTypeCommand,
		DocumentFormat: ssmtypes.DocumentFormatJson,
	})
	if err != nil {
		return err
	}
	defer func() {
		m.clients.SSM().DeleteDocument(context.TODO(), &ssm.DeleteDocumentInput{
			Name: doc.DocumentDescription.Name,
		})
	}()
	command, err := m.clients.SSM().SendCommand(context.TODO(), &ssm.SendCommandInput{
		DocumentName: doc.DocumentDescription.Name,
		InstanceIds:  instanceIds,
		Parameters: map[string][]string{
			"s3Destination": {fmt.Sprintf("s3://%s/node-logs/%s/%s/", opts.LogBucket, m.resourceID, phase)},
		},
	})
	if err != nil {
		return err
	}
	var errs []error
	for _, instanceId := range instanceIds {
		out, err := ssm.NewCommandExecutedWaiter(m.clients.SSM()).WaitForOutput(context.TODO(), &ssm.GetCommandInvocationInput{
			CommandId:  command.Command.CommandId,
			InstanceId: aws.String(instanceId),
		}, logCollectorSsmDocumentTimeout)
		if err != nil {
			errs = append(errs, err)
		} else {
			klog.Infof("log collection command for %s: %s", instanceId, out.Status)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	klog.Infof("gathered logs from nodes: %v", instanceIds)
	return nil
}

const logCollectorNodeDiagnosticTimeout = 5 * time.Minute

func (m *logManager) gatherLogsUsingNodeDiagnostic(k8sClient *k8sClient, opts *deployerOptions, phase deployerPhase) error {
	klog.Info("gathering logs using NodeDiagnostic...")
	nodes, err := k8sClient.clientset.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return err
	}
	if len(nodes.Items) == 0 {
		klog.Warning("no nodes to gather logs from!")
		return nil
	}
	instanceIds, err := getNodeInstanceIDs(nodes.Items)
	if err != nil {
		return err
	}
	var errs []error
	var nodeDiagnostics []unstructured.Unstructured
	for _, instanceId := range instanceIds {
		presignedPut, err := m.clients.S3Presign().PresignPutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(opts.LogBucket),
			Key:    aws.String(fmt.Sprintf("node-logs/%s/%s/%s.tar.gz", m.resourceID, phase, instanceId)),
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create presigned PUT for %s: %v", instanceId, err))
			continue
		}
		nodeDiagnostic := unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "eks.amazonaws.com/v1alpha1",
				"kind":       "NodeDiagnostic",
				"metadata": v1.ObjectMeta{
					Name: instanceId,
				},
				"spec": map[string]interface{}{
					"logCapture": map[string]interface{}{
						"destination": presignedPut.URL,
					},
				},
			},
		}
		if err := k8sClient.client.Create(context.TODO(), &nodeDiagnostic); err != nil {
			errs = append(errs, err)
		} else {
			nodeDiagnostics = append(nodeDiagnostics, nodeDiagnostic)
		}
	}
	outcomes, err := m.waitForNodeDiagnostics(k8sClient, nodeDiagnostics)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to wait for node diagnostics: %v", err))
	}
	for instanceId, reasons := range outcomes {
		for _, reason := range reasons {
			// consider SuccessWithErrors a success, this isn't high stakes
			if !slices.Contains([]string{"Success", "SuccessWithErrors"}, reason) {
				errs = append(errs, fmt.Errorf("node diagnostic outcome reason for %s: %s", instanceId, reason))
			}
		}
	}
	for _, nodeDiagnostic := range nodeDiagnostics {
		if err := k8sClient.client.Delete(context.TODO(), &nodeDiagnostic); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	klog.Infof("gathered logs from nodes: %v", instanceIds)
	return nil
}

// waitForNodeDiagnostics polls each node diagnostic until it reaches a terminal state, or the timeout is reached
// a map of node diagnostic names to their outcome reason(s) is returned if no error occurred
func (m *logManager) waitForNodeDiagnostics(k8sClient *k8sClient, nodeDiagnostics []unstructured.Unstructured) (map[string][]string, error) {
	outcomes := make(map[string][]string)
	err := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, logCollectorNodeDiagnosticTimeout, false, func(ctx context.Context) (done bool, err error) {
		for _, nodeDiagnostic := range nodeDiagnostics {
			objectKey := client.ObjectKeyFromObject(&nodeDiagnostic)
			if _, ok := outcomes[objectKey.Name]; ok {
				// we already have an outcome for this node diagnostic
				continue
			}
			if err := k8sClient.client.Get(ctx, objectKey, &nodeDiagnostic); err != nil {
				return false, fmt.Errorf("failed to get node diagnostic: %+v: %v", objectKey, err)
			}
			complete, reasons := m.isNodeDiagnosticComplete(&nodeDiagnostic)
			if !complete {
				continue
			}
			outcomes[objectKey.Name] = reasons
		}
		if len(outcomes) == len(nodeDiagnostics) {
			// we're done!
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return outcomes, nil
}

func (m *logManager) isNodeDiagnosticComplete(nodeDiagnostic *unstructured.Unstructured) (bool, []string) {
	captureStatuses, found, err := unstructured.NestedSlice(nodeDiagnostic.Object, "status", "captureStatuses")
	if err != nil {
		klog.Errorf("NodeDiagnostic captureStatuses does not match expected type: %+v", nodeDiagnostic)
		return false, nil
	}
	if !found {
		return false, nil
	}
	var reasons []string
	for _, captureStatus := range captureStatuses {
		captureStatusMap, ok := captureStatus.(map[string]interface{})
		if !ok {
			klog.Errorf("NodeDiagnostic captureStatus does not match expected type: %+v", nodeDiagnostic)
			return false, nil
		}
		reason, found, err := unstructured.NestedString(captureStatusMap, "state", "completed", "reason")
		if err != nil {
			klog.Errorf("NodeDiagnostic captureStatus.reason does not match expected type: %+v", nodeDiagnostic)
			return false, nil
		}
		if !found {
			return false, nil
		}
		reasons = append(reasons, reason)
	}
	return true, reasons
}
