package csi_ebs

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// getBoundPV returns a PV details.
func GetBoundPV(ts *tester, pvc *v1.PersistentVolumeClaim) (*v1.PersistentVolume, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	// Get new copy of the claim
	claim, err := ts.cfg.Client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Get(ctx, pvc.Name, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// Get the bound PV
	pv, err := ts.cfg.Client.CoreV1().PersistentVolumes().Get(ctx, claim.Spec.VolumeName, meta_v1.GetOptions{})
	cancel()
	return pv, err
}

// ExpandPVCSize expands PVC size
func ExpandPVCSize(ts *tester, origPVC *v1.PersistentVolumeClaim, size resource.Quantity) (*v1.PersistentVolumeClaim, error) {
	pvcName := origPVC.Name
	updatedPVC := origPVC.DeepCopy()
	const resizePollInterval = 2 * time.Second
	// Retry the update on error, until we hit a timeout.
	// TODO: Determine whether "retry with timeout" is appropriate here. Maybe we should only retry on version conflict.
	var lastUpdateError error
	waitErr := wait.PollImmediate(resizePollInterval, 30*time.Second, func() (bool, error) {
		var err error
		updatedPVC, err = ts.cfg.Client.CoreV1().PersistentVolumeClaims(origPVC.Namespace).Get(context.TODO(), pvcName, meta_v1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error fetching pvc %q for resizing: %v", pvcName, err)
		}
		updatedPVC.Spec.Resources.Requests[v1.ResourceStorage] = size
		updatedPVC, err = ts.cfg.Client.CoreV1().PersistentVolumeClaims(origPVC.Namespace).Update(context.TODO(), updatedPVC, meta_v1.UpdateOptions{})
		if err != nil {
			return false, fmt.Errorf("Error updating pvc pvcName: %v (%v)", err)
		}
		return true, nil
	})
	if waitErr == wait.ErrWaitTimeout {
		return nil, fmt.Errorf("timed out attempting to update PVC size. last update error: %v", lastUpdateError)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("failed to expand PVC size (check logs for error): %v", waitErr)
	}
	return updatedPVC, nil
}

func WaitForControllerVolumeResize(ts *tester, pvc *v1.PersistentVolumeClaim, timeout time.Duration) error {
	const resizePollInterval = 2 * time.Second
	pvName := pvc.Spec.VolumeName
	waitErr := wait.PollImmediate(resizePollInterval, timeout, func() (bool, error) {
		pvcSize := pvc.Spec.Resources.Requests[v1.ResourceStorage]

		pv, err := ts.cfg.Client.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, meta_v1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error fetching pv %q for resizing %v", pvName, err)
		}

		pvSize := pv.Spec.Capacity[v1.ResourceStorage]

		// If pv size is greater or equal to requested size that means controller resize is finished.
		if pvSize.Cmp(pvcSize) >= 0 {
			return true, nil
		}
		return false, nil
	})
	if waitErr != nil {
		return fmt.Errorf("error while waiting for controller resize to finish: %v", waitErr)
	}
	return nil
}
