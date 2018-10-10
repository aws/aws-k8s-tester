package prow

import (
	"strings"

	prowconfig "k8s.io/test-infra/prow/config"
)

func categorizeName(v string) (id string) {
	id = v

	// AWS Direct Connect https://aws.amazon.com/directconnect/
	id = strings.Replace(id, "-directconnect", "-connect", 1)
	// GCP Direct Connect https://cloud.google.com/interconnect/
	id = strings.Replace(id, "-gci", "-connect", 1)

	id = strings.Replace(id, "-gcp", "", 1)
	id = strings.Replace(id, "-gce", "", 1)
	id = strings.Replace(id, "-gke", "", 1)

	id = strings.Replace(id, "-aws", "", 1)
	id = strings.Replace(id, "-eks", "", 1)

	// "ci-ingress-aws-alb-e2e" to "ci-ingress-e2e"
	id = strings.Replace(id, "-alb", "", 1)

	return id
}

func categorizePresubmit(cfg prowconfig.Presubmit) (id string) {
	id = cfg.Name
	id2 := categorizeName(id)
	if id != id2 { // name was descriptive enough
		return id2
	}
	return id
}

func categorizePostsubmit(cfg prowconfig.Postsubmit) (id string) {
	id = cfg.Name
	id2 := categorizeName(id)
	if id != id2 { // name was descriptive enough
		return id2
	}
	return id
}

func categorizePeriodic(cfg prowconfig.Periodic) (id string) {
	id = cfg.Name
	id2 := categorizeName(id)
	if id != id2 { // name was descriptive enough
		return id2
	}
	return id
}
