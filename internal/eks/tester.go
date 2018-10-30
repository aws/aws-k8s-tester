package eks

import (
	"fmt"

	"github.com/aws/aws-k8s-tester/eksconfig"
	"github.com/aws/aws-k8s-tester/eksdeployer"
)

// NewTester returns a new EKS tester.
func NewTester(cfg *eksconfig.Config) (eksdeployer.Tester, error) {
	if err := cfg.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}
	switch cfg.TestMode {
	case "embedded":
		return newTesterEmbedded(cfg)
	case "aws-cli":
		return newTesterAWSCLI(cfg)
	default:
		return nil, fmt.Errorf("unknown TestMode %q", cfg.TestMode)
	}
}
