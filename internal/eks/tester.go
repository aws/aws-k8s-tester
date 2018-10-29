package eks

import (
	"fmt"

	"github.com/aws/awstester/eksconfig"
	"github.com/aws/awstester/eksdeployer"
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
