package kubernetesconfig

type CloudControllerManager struct {
	// Image is the container image name and tag for cloud-controller-manager to run as a static pod.
	Image string `json:"image"`
}

var defaultCloudControllerManager = CloudControllerManager{}

func newDefaultCloudControllerManager() *CloudControllerManager {
	copied := defaultCloudControllerManager
	return &copied
}
