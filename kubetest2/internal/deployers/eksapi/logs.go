package eksapi

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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
	// TODO: gather logs from Auto nodes
	if opts.AutoMode {
		klog.Info("--auto-mode was used, no logs will be gathered!")
		return nil
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
	klog.Info("gathering logs from nodes...")
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
