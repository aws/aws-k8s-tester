package deployer

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/aws/aws-k8s-tester/pkg/deployer/addons"
	"github.com/aws/aws-k8s-tester/pkg/deployer/types"
	"k8s.io/klog"
)

// Deployer implements the interface of kubetest2's Deployer
type Deployer struct {
	Configuration *types.DeployerConfiguration

	// Addons
	ManagedNodeGroups     *addons.ManagedNodeGroups
	IAMRoleServiceAccount *addons.IAMRoleServiceAccount
	ClusterAutoscaler     *addons.ClusterAutoscaler
}

// NewDeployer initializes the deployer
func NewDeployer(c *types.DeployerConfiguration) *Deployer {
	return &Deployer{
		Configuration: c,

		ManagedNodeGroups:     addons.NewManagedNodeGroups(),
		IAMRoleServiceAccount: addons.NewIAMRoleServiceAccount(),
		ClusterAutoscaler:     addons.NewClusterAutoscaler(),
	}

}

// GetInstallationOrder returns an ordering of addons to enable parallelism and dependency
func (d *Deployer) GetInstallationOrder() [][]addons.Addon {
	return [][]addons.Addon{{
		d.ManagedNodeGroups,
		d.IAMRoleServiceAccount,
	}, {
		d.ClusterAutoscaler,
	}}
}

// Up executes the Deployer, `aws-k8s-tester create eks`
func (d *Deployer) Up() error {
	// #1 Create Cluster
	// TODO

	// #2 Apply Addons in Groups to maintain dependency ordering
	for _, order := range d.GetInstallationOrder() {
		if err := d.runAsync(order, func(a addons.Addon) error {
			klog.Infof("Applying addon %s", reflect.TypeOf(a))
			return a.Apply(d.Configuration)
		}); err != nil {
			return fmt.Errorf("while applying addons, %w", err)
		}
	}

	return nil
}

// Down cleans up the Deployer, `aws-k8s-tester delete eks`
func (d *Deployer) Down() error {
	// #1 Uninstall Addons
	order := d.GetInstallationOrder()
	for index := range order {
		// Reverse the order for uninstall
		index := len(order) - 1 - index
		if err := d.runAsync(order[index], func(a addons.Addon) error {
			klog.Infof("Finalizing addon %s", reflect.TypeOf(a))
			return a.Apply(d.Configuration)
		}); err != nil {
			return fmt.Errorf("while finalizing addons, %w", err)
		}
	}

	// #2 Delete Cluster
	// TODO

	return nil
}

// IsUp is not currently used
func (d *Deployer) IsUp() (bool, error) {
	return false, nil
}

// Build is not currently used
func (d *Deployer) Build() error {
	return nil
}

// DumpClusterLogs is not current used
func (d *Deployer) DumpClusterLogs() error {
	return nil
}

// runAsync asynchronously executes a function over a slice of addons.
// If any function errors, the function will return with the error after all addons have executed
func (d *Deployer) runAsync(addons []addons.Addon, execute func(a addons.Addon) error) error {
	errors := make(chan error)
	done := make(chan bool)
	var wg sync.WaitGroup

	// Fire off addon functions
	for _, addon := range addons {
		if !addon.IsEnabled(d.Configuration) {
			klog.Infof("Skipping disabled addon %s", reflect.TypeOf(addon))
			continue
		}
		wg.Add(1)
		// Take a copy for the goroutine since addon will be mutated before it executes
		a := addon
		go func() {
			defer wg.Done()
			if err := execute(a); err != nil {
				errors <- err
			}
		}()
	}

	// Wait for all routines to exit and signal done
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for done or an error to occur
	select {
	case <-done:
		break
	case err := <-errors:
		close(errors)
		return err
	}
	return nil
}
