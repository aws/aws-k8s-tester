// Code generated by smithy-go-codegen DO NOT EDIT.

package iam

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Synchronizes the specified MFA device with its IAM resource object on the
// Amazon Web Services servers.
//
// For more information about creating and working with virtual MFA devices, see [Using a virtual MFA device]
// in the IAM User Guide.
//
// [Using a virtual MFA device]: https://docs.aws.amazon.com/IAM/latest/UserGuide/Using_VirtualMFA.html
func (c *Client) ResyncMFADevice(ctx context.Context, params *ResyncMFADeviceInput, optFns ...func(*Options)) (*ResyncMFADeviceOutput, error) {
	if params == nil {
		params = &ResyncMFADeviceInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "ResyncMFADevice", params, optFns, c.addOperationResyncMFADeviceMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*ResyncMFADeviceOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type ResyncMFADeviceInput struct {

	// An authentication code emitted by the device.
	//
	// The format for this parameter is a sequence of six digits.
	//
	// This member is required.
	AuthenticationCode1 *string

	// A subsequent authentication code emitted by the device.
	//
	// The format for this parameter is a sequence of six digits.
	//
	// This member is required.
	AuthenticationCode2 *string

	// Serial number that uniquely identifies the MFA device.
	//
	// This parameter allows (through its [regex pattern]) a string of characters consisting of upper
	// and lowercase alphanumeric characters with no spaces. You can also include any
	// of the following characters: _+=,.@-
	//
	// [regex pattern]: http://wikipedia.org/wiki/regex
	//
	// This member is required.
	SerialNumber *string

	// The name of the user whose MFA device you want to resynchronize.
	//
	// This parameter allows (through its [regex pattern]) a string of characters consisting of upper
	// and lowercase alphanumeric characters with no spaces. You can also include any
	// of the following characters: _+=,.@-
	//
	// [regex pattern]: http://wikipedia.org/wiki/regex
	//
	// This member is required.
	UserName *string

	noSmithyDocumentSerde
}

type ResyncMFADeviceOutput struct {
	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationResyncMFADeviceMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsquery_serializeOpResyncMFADevice{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsquery_deserializeOpResyncMFADevice{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "ResyncMFADevice"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = addClientRequestID(stack); err != nil {
		return err
	}
	if err = addComputeContentLength(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = addComputePayloadSHA256(stack); err != nil {
		return err
	}
	if err = addRetry(stack, options); err != nil {
		return err
	}
	if err = addRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = addRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addSpanRetryLoop(stack, options); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addTimeOffsetBuild(stack, c); err != nil {
		return err
	}
	if err = addUserAgentRetryMode(stack, options); err != nil {
		return err
	}
	if err = addOpResyncMFADeviceValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opResyncMFADevice(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addRecursionDetection(stack); err != nil {
		return err
	}
	if err = addRequestIDRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	if err = addSpanInitializeStart(stack); err != nil {
		return err
	}
	if err = addSpanInitializeEnd(stack); err != nil {
		return err
	}
	if err = addSpanBuildRequestStart(stack); err != nil {
		return err
	}
	if err = addSpanBuildRequestEnd(stack); err != nil {
		return err
	}
	return nil
}

func newServiceMetadataMiddleware_opResyncMFADevice(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "ResyncMFADevice",
	}
}
