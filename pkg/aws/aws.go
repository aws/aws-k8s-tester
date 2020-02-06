package aws

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
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

	// ResolverURL is a custom resolver URL.
	ResolverURL string
	// SigningName is the API signing name.
	SigningName string
}

// New creates a new AWS session.
// Specify a custom endpoint for tests.
func New(cfg *Config) (ss *session.Session, stsOutput *sts.GetCallerIdentityOutput, awsCredsPath string, err error) {
	if cfg == nil {
		return nil, nil, "", errors.New("got empty config")
	}
	if cfg.Logger == nil {
		return nil, nil, "", fmt.Errorf("missing logger")
	}
	if cfg.Region == "" {
		return nil, nil, "", fmt.Errorf("missing region")
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
			return nil, nil, "", errors.New("cannot find AWS credentials")
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

	var stsSession *session.Session
	stsSession, err = session.NewSession(&awsConfig)
	if err != nil {
		return nil, nil, "", err
	}
	stsSvc := sts.New(stsSession)
	stsOutput, err = stsSvc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, nil, "", err
	}
	cfg.Logger.Info(
		"creating AWS session",
		zap.String("account-id", *stsOutput.Account),
		zap.String("user-id", *stsOutput.UserId),
		zap.String("arn", *stsOutput.Arn),
	)

	resolver := endpoints.DefaultResolver()

	if cfg.ResolverURL != "" && cfg.SigningName == "" {
		return nil, nil, "", fmt.Errorf("got empty signing name for resolver %q", cfg.ResolverURL)
	}

	// support test endpoint (e.g. https://api.beta.us-west-2.wesley.amazonaws.com)
	if cfg.ResolverURL != "" {
		cfg.Logger.Info(
			"setting custom resolver",
			zap.String("resolver-url", cfg.ResolverURL),
			zap.String("signing-name", cfg.SigningName),
		)
		resolver = endpoints.ResolverFunc(func(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
			if service == "eks" {
				return endpoints.ResolvedEndpoint{
					URL:         cfg.ResolverURL,
					SigningName: cfg.SigningName,
				}, nil
			}
			return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
		})
	}
	awsConfig.EndpointResolver = resolver

	ss, err = session.NewSession(&awsConfig)
	if err != nil {
		return nil, nil, "", err
	}
	return ss, stsOutput, awsCredsPath, err
}
