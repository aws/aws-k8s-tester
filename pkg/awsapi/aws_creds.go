package awsapi

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/go-ini/ini"
)

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
