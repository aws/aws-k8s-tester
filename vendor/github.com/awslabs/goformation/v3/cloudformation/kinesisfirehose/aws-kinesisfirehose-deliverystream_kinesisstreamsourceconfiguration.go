package kinesisfirehose

import (
	"github.com/awslabs/goformation/v3/cloudformation/policies"
)

// DeliveryStream_KinesisStreamSourceConfiguration AWS CloudFormation Resource (AWS::KinesisFirehose::DeliveryStream.KinesisStreamSourceConfiguration)
// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-kinesisfirehose-deliverystream-kinesisstreamsourceconfiguration.html
type DeliveryStream_KinesisStreamSourceConfiguration struct {

	// KinesisStreamARN AWS CloudFormation Property
	// Required: true
	// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-kinesisfirehose-deliverystream-kinesisstreamsourceconfiguration.html#cfn-kinesisfirehose-deliverystream-kinesisstreamsourceconfiguration-kinesisstreamarn
	KinesisStreamARN string `json:"KinesisStreamARN,omitempty"`

	// RoleARN AWS CloudFormation Property
	// Required: true
	// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-kinesisfirehose-deliverystream-kinesisstreamsourceconfiguration.html#cfn-kinesisfirehose-deliverystream-kinesisstreamsourceconfiguration-rolearn
	RoleARN string `json:"RoleARN,omitempty"`

	// _deletionPolicy represents a CloudFormation DeletionPolicy
	_deletionPolicy policies.DeletionPolicy

	// _dependsOn stores the logical ID of the resources to be created before this resource
	_dependsOn []string

	// _metadata stores structured data associated with this resource
	_metadata map[string]interface{}
}

// AWSCloudFormationType returns the AWS CloudFormation resource type
func (r *DeliveryStream_KinesisStreamSourceConfiguration) AWSCloudFormationType() string {
	return "AWS::KinesisFirehose::DeliveryStream.KinesisStreamSourceConfiguration"
}

// DependsOn returns a slice of logical ID names this resource depends on.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-dependson.html
func (r *DeliveryStream_KinesisStreamSourceConfiguration) DependsOn() []string {
	return r._dependsOn
}

// SetDependsOn specify that the creation of this resource follows another.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-dependson.html
func (r *DeliveryStream_KinesisStreamSourceConfiguration) SetDependsOn(dependencies []string) {
	r._dependsOn = dependencies
}

// Metadata returns the metadata associated with this resource.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-metadata.html
func (r *DeliveryStream_KinesisStreamSourceConfiguration) Metadata() map[string]interface{} {
	return r._metadata
}

// SetMetadata enables you to associate structured data with this resource.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-metadata.html
func (r *DeliveryStream_KinesisStreamSourceConfiguration) SetMetadata(metadata map[string]interface{}) {
	r._metadata = metadata
}

// DeletionPolicy returns the AWS CloudFormation DeletionPolicy to this resource
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-deletionpolicy.html
func (r *DeliveryStream_KinesisStreamSourceConfiguration) DeletionPolicy() policies.DeletionPolicy {
	return r._deletionPolicy
}

// SetDeletionPolicy applies an AWS CloudFormation DeletionPolicy to this resource
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-deletionpolicy.html
func (r *DeliveryStream_KinesisStreamSourceConfiguration) SetDeletionPolicy(policy policies.DeletionPolicy) {
	r._deletionPolicy = policy
}
