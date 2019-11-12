package appsync

import (
	"github.com/awslabs/goformation/v3/cloudformation/policies"
)

// GraphQLApi_UserPoolConfig AWS CloudFormation Resource (AWS::AppSync::GraphQLApi.UserPoolConfig)
// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-appsync-graphqlapi-userpoolconfig.html
type GraphQLApi_UserPoolConfig struct {

	// AppIdClientRegex AWS CloudFormation Property
	// Required: false
	// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-appsync-graphqlapi-userpoolconfig.html#cfn-appsync-graphqlapi-userpoolconfig-appidclientregex
	AppIdClientRegex string `json:"AppIdClientRegex,omitempty"`

	// AwsRegion AWS CloudFormation Property
	// Required: false
	// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-appsync-graphqlapi-userpoolconfig.html#cfn-appsync-graphqlapi-userpoolconfig-awsregion
	AwsRegion string `json:"AwsRegion,omitempty"`

	// DefaultAction AWS CloudFormation Property
	// Required: false
	// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-appsync-graphqlapi-userpoolconfig.html#cfn-appsync-graphqlapi-userpoolconfig-defaultaction
	DefaultAction string `json:"DefaultAction,omitempty"`

	// UserPoolId AWS CloudFormation Property
	// Required: false
	// See: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-appsync-graphqlapi-userpoolconfig.html#cfn-appsync-graphqlapi-userpoolconfig-userpoolid
	UserPoolId string `json:"UserPoolId,omitempty"`

	// _deletionPolicy represents a CloudFormation DeletionPolicy
	_deletionPolicy policies.DeletionPolicy

	// _dependsOn stores the logical ID of the resources to be created before this resource
	_dependsOn []string

	// _metadata stores structured data associated with this resource
	_metadata map[string]interface{}
}

// AWSCloudFormationType returns the AWS CloudFormation resource type
func (r *GraphQLApi_UserPoolConfig) AWSCloudFormationType() string {
	return "AWS::AppSync::GraphQLApi.UserPoolConfig"
}

// DependsOn returns a slice of logical ID names this resource depends on.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-dependson.html
func (r *GraphQLApi_UserPoolConfig) DependsOn() []string {
	return r._dependsOn
}

// SetDependsOn specify that the creation of this resource follows another.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-dependson.html
func (r *GraphQLApi_UserPoolConfig) SetDependsOn(dependencies []string) {
	r._dependsOn = dependencies
}

// Metadata returns the metadata associated with this resource.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-metadata.html
func (r *GraphQLApi_UserPoolConfig) Metadata() map[string]interface{} {
	return r._metadata
}

// SetMetadata enables you to associate structured data with this resource.
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-metadata.html
func (r *GraphQLApi_UserPoolConfig) SetMetadata(metadata map[string]interface{}) {
	r._metadata = metadata
}

// DeletionPolicy returns the AWS CloudFormation DeletionPolicy to this resource
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-deletionpolicy.html
func (r *GraphQLApi_UserPoolConfig) DeletionPolicy() policies.DeletionPolicy {
	return r._deletionPolicy
}

// SetDeletionPolicy applies an AWS CloudFormation DeletionPolicy to this resource
// see: https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-attribute-deletionpolicy.html
func (r *GraphQLApi_UserPoolConfig) SetDeletionPolicy(policy policies.DeletionPolicy) {
	r._deletionPolicy = policy
}
