// Package aws implements wrappers for AWS API calls.
package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
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

	// Partition is an AWS partition (default "aws").
	Partition string
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
	if cfg.Partition == "" {
		return nil, nil, "", fmt.Errorf("missing partition")
	}
	if cfg.Region == "" {
		return nil, nil, "", fmt.Errorf("missing region")
	}
	if cfg.ResolverURL != "" && cfg.SigningName == "" {
		return nil, nil, "", fmt.Errorf("got empty signing name for resolver %q", cfg.ResolverURL)
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
	// See https://pkg.go.dev/github.com/aws/aws-sdk-go/aws/credentials#SharedCredentialsProvider.
	// See https://pkg.go.dev/github.com/aws/aws-sdk-go/aws/session#hdr-Environment_Variables.
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
			cfg.Logger.Info("cannot create a session from env vars")
		} else {
			cfg.Logger.Info("creating a session from env vars")
		}
	}

	if cfg.DebugAPICalls {
		lvl := aws.LogDebug |
			aws.LogDebugWithEventStreamBody |
			aws.LogDebugWithHTTPBody |
			aws.LogDebugWithRequestRetries |
			aws.LogDebugWithRequestErrors
		awsConfig.LogLevel = &lvl
	}

	var partition endpoints.Partition
	switch cfg.Partition {
	case endpoints.AwsPartitionID:
		partition = endpoints.AwsPartition()
	case endpoints.AwsCnPartitionID:
		partition = endpoints.AwsCnPartition()
	case endpoints.AwsUsGovPartitionID:
		partition = endpoints.AwsUsGovPartition()
	case endpoints.AwsIsoPartitionID:
		partition = endpoints.AwsIsoPartition()
	case endpoints.AwsIsoBPartitionID:
		partition = endpoints.AwsIsoBPartition()
	default:
		return nil, nil, "", fmt.Errorf("unknown partition %q", cfg.Partition)
	}
	regions := partition.Regions()
	region, ok := regions[cfg.Region]
	if !ok {
		return nil, nil, "", fmt.Errorf("region %q for partition %q not found in %+v", cfg.Region, cfg.Partition, regions)
	}
	stsEndpoint, err := region.ResolveEndpoint(endpoints.StsServiceID)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to resolve endpoints for sts %q (%v)", cfg.Region, err)
	}
	stsConfig := awsConfig
	stsConfig.STSRegionalEndpoint = endpoints.RegionalSTSEndpoint
	var stsSession *session.Session
	stsSession, err = session.NewSession(&stsConfig)
	if err != nil {
		return nil, nil, "", err
	}
	/*
		iamSvc := iam.New(stsSession)
		if _, err = iamSvc.SetSecurityTokenServicePreferences(&iam.SetSecurityTokenServicePreferencesInput{
			GlobalEndpointTokenVersion: aws.String("v2Token"),
		}); err != nil {
			cfg.Logger.Warn("failed to enable v2 security token", zap.Error(err))
		}
	*/
	stsSvc := sts.New(stsSession)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	stsOutput, err = stsSvc.GetCallerIdentityWithContext(
		ctx,
		&sts.GetCallerIdentityInput{},
		request.WithLogLevel(aws.LogDebug),
		request.WithResponseReadTimeout(15*time.Second),
	)
	cancel()
	if err != nil {
		cfg.Logger.Warn("failed to get sts caller identity",
			zap.String("partition", cfg.Partition),
			zap.String("region", cfg.Region),
			zap.String("region-id", region.ID()),
			zap.String("region-description", region.Description()),
			zap.String("region-resolved-sts-endpoint", stsEndpoint.URL),
			zap.Error(err),
		)
	} else {
		cfg.Logger.Info("successfully get sts caller identity",
			zap.String("partition", cfg.Partition),
			zap.String("region", cfg.Region),
			zap.String("region-id", region.ID()),
			zap.String("region-description", region.Description()),
			zap.String("region-resolved-sts-endpoint", stsEndpoint.URL),
			zap.String("account-id", aws.StringValue(stsOutput.Account)),
			zap.String("user-id", aws.StringValue(stsOutput.UserId)),
			zap.String("arn", aws.StringValue(stsOutput.Arn)),
		)
	}

	resolver := endpoints.DefaultResolver()
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

	cfg.Logger.Info(
		"creating AWS session",
		zap.String("partition", cfg.Partition),
		zap.String("region", cfg.Region),
	)
	ss, err = session.NewSession(&awsConfig)
	if err != nil {
		return nil, nil, "", err
	}
	return ss, stsOutput, awsCredsPath, err
}
