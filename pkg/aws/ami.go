package aws

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"sigs.k8s.io/yaml"
)

// AMI represents AMI.
type AMI struct {
	ARN              string    `json:"arn"`
	Name             string    `json:"name"`
	Version          int64     `json:"version"`
	LastModifiedDate time.Time `json:"last-modified-date"`
	SchemaVersion    string    `json:"schema_version,omitempty"`
	ImageID          string    `json:"image_id,omitempty"`
	ImageName        string    `json:"image_name,omitempty"`
}

// FetchAMI gets AMI from the SSM parameter key.
func FetchAMI(sa ssmiface.SSMAPI, key string) (*AMI, error) {
	so, err := sa.GetParameters(&ssm.GetParametersInput{
		Names: aws.StringSlice([]string{key})})
	if err != nil {
		return nil, err
	}
	if len(so.Parameters) != 1 {
		return nil, fmt.Errorf("unexpected parameters received %+v", so.Parameters)
	}
	param := so.Parameters[0]
	ami := new(AMI)
	v := aws.StringValue(param.Value)
	switch {
	case strings.HasPrefix(v, "ami-"):
		ami.ImageID = v
	case strings.Contains(v, "schema_version"):
		err = yaml.Unmarshal([]byte(v), ami)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("cannot parse %s", v)
	}

	ami.ARN = aws.StringValue(param.ARN)
	ami.Name = aws.StringValue(param.Name)
	ami.Version = aws.Int64Value(param.Version)
	ami.LastModifiedDate = aws.TimeValue(param.LastModifiedDate)
	return ami, nil
}
