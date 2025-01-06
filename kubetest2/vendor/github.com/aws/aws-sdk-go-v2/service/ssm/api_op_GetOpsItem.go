// Code generated by smithy-go-codegen DO NOT EDIT.

package ssm

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Get information about an OpsItem by using the ID. You must have permission in
// Identity and Access Management (IAM) to view information about an OpsItem. For
// more information, see [Set up OpsCenter]in the Amazon Web Services Systems Manager User Guide.
//
// Operations engineers and IT professionals use Amazon Web Services Systems
// Manager OpsCenter to view, investigate, and remediate operational issues
// impacting the performance and health of their Amazon Web Services resources. For
// more information, see [Amazon Web Services Systems Manager OpsCenter]in the Amazon Web Services Systems Manager User Guide.
//
// [Amazon Web Services Systems Manager OpsCenter]: https://docs.aws.amazon.com/systems-manager/latest/userguide/OpsCenter.html
// [Set up OpsCenter]: https://docs.aws.amazon.com/systems-manager/latest/userguide/OpsCenter-setup.html
func (c *Client) GetOpsItem(ctx context.Context, params *GetOpsItemInput, optFns ...func(*Options)) (*GetOpsItemOutput, error) {
	if params == nil {
		params = &GetOpsItemInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "GetOpsItem", params, optFns, c.addOperationGetOpsItemMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*GetOpsItemOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type GetOpsItemInput struct {

	// The ID of the OpsItem that you want to get.
	//
	// This member is required.
	OpsItemId *string

	// The OpsItem Amazon Resource Name (ARN).
	OpsItemArn *string

	noSmithyDocumentSerde
}

type GetOpsItemOutput struct {

	// The OpsItem.
	OpsItem *types.OpsItem

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationGetOpsItemMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsjson11_serializeOpGetOpsItem{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsjson11_deserializeOpGetOpsItem{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "GetOpsItem"); err != nil {
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
	if err = addOpGetOpsItemValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opGetOpsItem(options.Region), middleware.Before); err != nil {
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

func newServiceMetadataMiddleware_opGetOpsItem(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "GetOpsItem",
	}
}
