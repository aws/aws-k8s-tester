package awsapi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-k8s-tester/pkg/fileutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-ini/ini"
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

type credentialsProvider struct {
	credentialsPath string
	refreshDuration time.Duration
	roleArn         string
	awsRefreshTime  time.Time
	nextReadTime    time.Time
}

func newCreds(credentialsPath string, refreshDuration time.Duration) (*credentials.Credentials, error) {
	// Credentials Path must be provided
	if credentialsPath == "" {
		return nil, awserr.New("EKSCredentialsPathRequired", "Credentials File Path is required!", nil)
	}

	// Credentials Path must exist!
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		return nil, awserr.New("EKSCredentialsFileNotFound", "Credentials File Not Found.", nil)
	}

	// Duration cannot be more than one hour and less than one second
	if refreshDuration < time.Second || refreshDuration > time.Hour {
		refreshDuration = 15 * time.Minute
	}

	p := credentialsProvider{
		credentialsPath: credentialsPath,
		refreshDuration: refreshDuration,
	}
	return credentials.NewCredentials(&p), nil
}

func (p *credentialsProvider) Retrieve() (credentials.Value, error) {
	creds, awsRefreshTime, roleArn, err := p.loadFromDisk()
	if err != nil {
		return credentials.Value{ProviderName: "EKSCredentialsProvider"}, err
	}

	p.awsRefreshTime = awsRefreshTime
	p.roleArn = roleArn
	p.nextReadTime = time.Now().UTC().Add(p.refreshDuration)
	return creds, nil
}

func (p *credentialsProvider) IsExpired() bool {
	now := time.Now().UTC()
	return now.After(p.nextReadTime) || now.After(p.awsRefreshTime)
}

func (p *credentialsProvider) AwsRefreshTime() time.Time {
	return p.awsRefreshTime
}

func (p *credentialsProvider) RemainingUntilAwsRefreshTime() time.Duration {
	now := time.Now().UTC()
	return p.awsRefreshTime.Sub(now)
}

func (p *credentialsProvider) RemainingSecondsUntilAwsRefreshTime() float64 {
	return p.RemainingUntilAwsRefreshTime().Seconds()
}

func (p *credentialsProvider) RoleArn() string {
	return p.roleArn
}

// use default profile
// https://github.com/aws/aws-sdk-go/blob/master/aws/credentials/shared_credentials_provider.go
func (p *credentialsProvider) loadFromDisk() (credentials.Value, time.Time, string, error) {
	config, err := ini.Load(p.credentialsPath)
	if err != nil {
		return credentials.Value{ProviderName: "EKSCredentialsProvider"},
			time.Now().UTC(),
			"",
			awserr.New("EKSCredentialsLoadFailure", "Failed to load credentials file!", err)
	}
	iniProfile, err := config.GetSection("default")
	if err != nil {
		return credentials.Value{ProviderName: "EKSCredentialsProvider"},
			time.Now().UTC(),
			"",
			awserr.New("EKSCredentialsLoadFailure", "Failed to load default DefaultProfile", err)
	}

	id, err := iniProfile.GetKey("aws_access_key_id")
	if err != nil {
		return credentials.Value{ProviderName: "EKSCredentialsProvider"},
			time.Now().UTC(),
			"",
			awserr.New("EKSCredentialsMissingAccessKey",
				fmt.Sprintf("Credentials %s in %s did not contain aws_access_key_id", "default", p.credentialsPath),
				err)
	}

	secret, err := iniProfile.GetKey("aws_secret_access_key")
	if err != nil {
		return credentials.Value{ProviderName: "EKSCredentialsProvider"},
			time.Now().UTC(),
			"",
			awserr.New("EKSCredentialsMissingSecret",
				fmt.Sprintf("Credentials %s in %s did not contain aws_secret_access_key", "default", p.credentialsPath),
				err)
	}

	token, err := iniProfile.GetKey("aws_session_token")
	if err != nil {
		return credentials.Value{ProviderName: "EKSCredentialsProvider"},
			time.Now().UTC(),
			"",
			awserr.New("EKSCredentialsMissingToken",
				fmt.Sprintf("Credentials %s in %s did not contain aws_session_token", "default", p.credentialsPath),
				err)
	}

	// Role ARN may be empty. Default to empty string if not found
	roleArn := strings.TrimSpace(iniProfile.Key("aws_role_arn").String())
	// Parse AWS Refresh Time
	awsRefreshTime := parseAwsRefreshTime(iniProfile)

	return credentials.Value{
		AccessKeyID:     id.String(),
		SecretAccessKey: secret.String(),
		SessionToken:    token.String(),
		ProviderName:    "EKSCredentialsProvider",
	}, awsRefreshTime, roleArn, nil
}

func parseAwsRefreshTime(iniProfile *ini.Section) time.Time {
	v := time.Now().UTC().Add(15 * time.Minute)
	rs := strings.TrimSpace(iniProfile.Key("aws_refresh_time").String())
	if rs != "" {
		n, err := strconv.ParseInt(rs, 10, 64)
		if err == nil && n > 0 {
			v = time.Unix(n, 0).In(time.UTC)
		}
	}
	return v
}
