package awsapi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
	"k8s.io/client-go/util/homedir"
)

// Config defines a top-level AWS API configuration to create a session.
type Config struct {
	// Logger is the log object.
	Logger *zap.Logger

	// DebugAPICalls is true to log all AWS API call debugging messages.
	DebugAPICalls bool

	// Region is a separate AWS geographic area for EKS service.
	// Each AWS Region has multiple, isolated locations known as Availability Zones.
	Region string

	// CustomEndpoint is a custom endpoint for pre-release versions of the service.
	// TODO: support EKS testing endpoint
	// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#custom-endpoint
	CustomEndpoint string
}

// New creates a new AWS session.
// Specify a custom endpoint for tests.
func New(cfg *Config) (ss *session.Session, awsCredsPath string, err error) {
	if cfg == nil {
		return nil, "", errors.New("got empty config")
	}
	if cfg.Logger == nil {
		return nil, "", fmt.Errorf("missing logger")
	}
	if cfg.Region == "" {
		return nil, "", fmt.Errorf("missing region")
	}

	awsConfig := aws.Config{
		Region:                        aws.String(cfg.Region),
		CredentialsChainVerboseErrors: aws.Bool(true),
		Logger:                        toLogger(cfg.Logger),
	}

	// Credential is the path to the shared credentials file.
	//
	// If empty will look for "AWS_SHARED_CREDENTIALS_FILE" env variable. If the
	// env value is empty will default to current user's home directory.
	// Linux/OSX: "$HOME/.aws/credentials"
	// Windows:   "%USERPROFILE%\.aws\credentials"
	//
	// See https://godoc.org/github.com/aws/aws-sdk-go/aws/credentials#SharedCredentialsProvider.
	// See https://godoc.org/github.com/aws/aws-sdk-go/aws/session#hdr-Environment_Variables.
	awsCredsPath = filepath.Join(homedir.HomeDir(), ".aws", "credentials")
	if os.Getenv("AWS_SHARED_CREDENTIALS_FILE") != "" {
		awsCredsPath = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	}
	if fileutil.Exist(awsCredsPath) {
		cfg.Logger.Info("creating session from AWS cred file", zap.String("path", awsCredsPath))
		// TODO: support temporary credentials with refresh mechanism
	} else {
		cfg.Logger.Info("cannot find AWS cred file", zap.String("path", awsCredsPath))
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" ||
			os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
			return nil, "", errors.New("cannot find AWS credentials")
		}
		cfg.Logger.Info("creating session from env vars")
	}

	if cfg.DebugAPICalls {
		lvl := aws.LogDebug |
			aws.LogDebugWithEventStreamBody |
			aws.LogDebugWithHTTPBody |
			aws.LogDebugWithRequestRetries |
			aws.LogDebugWithRequestErrors
		awsConfig.LogLevel = &lvl
	}

	// TODO: support test endpoint
	// https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#custom-endpoint
	if cfg.CustomEndpoint != "" {
		awsConfig.Endpoint = aws.String(cfg.CustomEndpoint)
	}

	ss, err = session.NewSession(&awsConfig)
	if err != nil {
		return nil, "", err
	}
	return ss, awsCredsPath, err
}
