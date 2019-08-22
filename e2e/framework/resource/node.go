package resource

import (
	"context"
	"time"

	"github.com/aws/aws-k8s-tester/e2e/framework/utils"

	log "github.com/cihub/seelog"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	// poll is how often to Poll pods, nodes and claims.
	poll = 2 * time.Second

	// singleCallTimeout is how long to try single API calls (like 'get' or 'list'). Used to prevent
	// transient failures from failing tests.
	// TODO: client should not apply this timeout to Watch calls. Increased from 30s until that is fixed.
	singleCallTimeout = 5 * time.Minute

	// TaintNodeNotReady will be added when node is not ready
	// and feature-gate for TaintBasedEvictions flag is enabled,
	// and removed when node becomes ready.
	TaintNodeNotReady = "node.kubernetes.io/not-ready"

	// TaintNodeUnreachable will be added when node becomes unreachable
	// (corresponding to NodeReady status ConditionUnknown)
	// and feature-gate for TaintBasedEvictions flag is enabled,
	// and removed when node becomes reachable (NodeReady status ConditionTrue).
	TaintNodeUnreachable = "node.kubernetes.io/unreachable"
)

var (
	// UnreachableTaintTemplate is the taint for when a node becomes unreachable.
	UnreachableTaintTemplate = &corev1.Taint{
		Key:    TaintNodeUnreachable,
		Effect: v1.TaintEffectNoExecute,
	}
	// NotReadyTaintTemplate is the taint for when a node is not ready for
	// executing pods
	NotReadyTaintTemplate = &corev1.Taint{
		Key:    TaintNodeNotReady,
		Effect: v1.TaintEffectNoExecute,
	}
)

type NodeManager struct {
	cs kubernetes.Interface
}

func NewNodeManager(cs kubernetes.Interface) *NodeManager {
	return &NodeManager{
		cs: cs,
	}
}

func (m *NodeManager) WaitNodeExists(ctx context.Context, n *corev1.Node) (*corev1.Node, error) {
	var (
		observedN *corev1.Node
		err       error
	)

	return observedN, wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		observedN, err = m.cs.CoreV1().Nodes().Get(n.Name, metav1.GetOptions{})
		if err != nil {
			if serr, ok := err.(*errors.StatusError); ok {
				switch serr.ErrStatus.Reason {
				case "NotFound":
					return false, nil
				default:
					return false, err
				}
			}
			return false, err
		}
		return true, nil
	}, ctx.Done())
}

func (m *NodeManager) WaitNodeReady(ctx context.Context, n *corev1.Node) (*corev1.Node, error) {
	var (
		observedN *corev1.Node
		err       error
	)

	return observedN, wait.PollImmediateUntil(utils.PollIntervalShort, func() (bool, error) {
		observedN, err = m.cs.CoreV1().Nodes().Get(n.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return isNodeReady(observedN), nil
	}, ctx.Done())
}

func isNodeReady(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			// Check if the node has taints UnreachableTaintTemplate or NotReadyTaintTemplate
			hasTaints := false
			taints := node.Spec.Taints
			for _, taint := range taints {
				if taint.MatchTaint(UnreachableTaintTemplate) || taint.MatchTaint(NotReadyTaintTemplate) {
					log.Debug("node (%s) has taint %s", node.Name, taint.String())
					hasTaints = true
					break
				}
			}
			if !hasTaints && condition.Status == v1.ConditionTrue {
				log.Debug("node (%s) has ready status %s", node.Name, condition.Status)
				return true
			}
			log.Debug("node (%s) has ready status %s", node.Name, condition.Status)
		}
	}
	return false
}

// // TODO: better to change to a easy read name
// func isNodeConditionSetAsExpected(node *v1.Node, conditionType v1.NodeConditionType, wantTrue, silent bool) bool {
// 	// Check the node readiness condition (logging all).
// 	for _, cond := range node.Status.Conditions {
// 		// Ensure that the condition type and the status matches as desired.
// 		if cond.Type == conditionType {
// 			// For NodeReady condition we need to check Taints as well
// 			if cond.Type == v1.NodeReady {
// 				hasNodeControllerTaints := false
// 				// For NodeReady we need to check if Taints are gone as well
// 				taints := node.Spec.Taints
// 				for _, taint := range taints {

// 					if taint.MatchTaint(UnreachableTaintTemplate) || taint.MatchTaint(NotReadyTaintTemplate) {
// 						hasNodeControllerTaints = true
// 						break
// 					}
// 				}
// 				if wantTrue {
// 					if (cond.Status == v1.ConditionTrue) && !hasNodeControllerTaints {
// 						return true
// 					}
// 					msg := ""
// 					if !hasNodeControllerTaints {
// 						msg = fmt.Sprintf("Condition %s of node %s is %v instead of %t. Reason: %v, message: %v",
// 							conditionType, node.Name, cond.Status == v1.ConditionTrue, wantTrue, cond.Reason, cond.Message)
// 					}
// 					msg = fmt.Sprintf("Condition %s of node %s is %v, but Node is tainted by NodeController with %v. Failure",
// 						conditionType, node.Name, cond.Status == v1.ConditionTrue, taints)
// 					if !silent {
// 						log.Debugf(msg)
// 					}
// 					return false
// 				}
// 				// TODO: check if the Node is tainted once we enable NC notReady/unreachable taints by default
// 				if cond.Status != v1.ConditionTrue {
// 					return true
// 				}
// 				if !silent {
// 					log.Debugf("Condition %s of node %s is %v instead of %t. Reason: %v, message: %v",
// 						conditionType, node.Name, cond.Status == v1.ConditionTrue, wantTrue, cond.Reason, cond.Message)
// 				}
// 				return false
// 			}
// 			if (wantTrue && (cond.Status == v1.ConditionTrue)) || (!wantTrue && (cond.Status != v1.ConditionTrue)) {
// 				return true
// 			}
// 			if !silent {
// 				log.Debugf("Condition %s of node %s is %v instead of %t. Reason: %v, message: %v",
// 					conditionType, node.Name, cond.Status == v1.ConditionTrue, wantTrue, cond.Reason, cond.Message)
// 			}
// 			return false
// 		}

// 	}
// 	if !silent {
// 		log.Debugf("Couldn't find condition %v on node %v", conditionType, node.Name)
// 	}
// 	return false
// }
