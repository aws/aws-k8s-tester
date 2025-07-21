package manifests

import (
	_ "embed"
)

var (
	//go:embed assets/nvidia-device-plugin.yaml
	NvidiaDevicePluginManifest []byte
	//go:embed assets/mpi-operator.yaml
	MpiOperatorManifest []byte

	//go:embed assets/efa-device-plugin.yaml
	EfaDevicePluginManifest []byte

	//go:embed assets/k8s-neuron-device-plugin-rbac.yml
	NeuronDevicePluginRbacManifest []byte
	//go:embed assets/k8s-neuron-device-plugin.yml
	NeuronDevicePluginManifest []byte

	//go:embed assets/dcgm-exporter.yaml
	DCGMExporterManifest []byte
)
