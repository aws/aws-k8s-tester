// Code generated by smithy-go-codegen DO NOT EDIT.

package ec2

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Creates a network interface in the specified subnet. The number of IP addresses
// you can assign to a network interface varies by instance type. For more
// information, see IP Addresses Per ENI Per Instance Type (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html#AvailableIpPerENI)
// in the Amazon Virtual Private Cloud User Guide. For more information about
// network interfaces, see Elastic network interfaces (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-eni.html)
// in the Amazon Elastic Compute Cloud User Guide.
func (c *Client) CreateNetworkInterface(ctx context.Context, params *CreateNetworkInterfaceInput, optFns ...func(*Options)) (*CreateNetworkInterfaceOutput, error) {
	if params == nil {
		params = &CreateNetworkInterfaceInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "CreateNetworkInterface", params, optFns, c.addOperationCreateNetworkInterfaceMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*CreateNetworkInterfaceOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type CreateNetworkInterfaceInput struct {

	// The ID of the subnet to associate with the network interface.
	//
	// This member is required.
	SubnetId *string

	// Unique, case-sensitive identifier that you provide to ensure the idempotency of
	// the request. For more information, see Ensuring Idempotency (https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Run_Instance_Idempotency.html)
	// .
	ClientToken *string

	// A connection tracking specification for the network interface.
	ConnectionTrackingSpecification *types.ConnectionTrackingSpecificationRequest

	// A description for the network interface.
	Description *string

	// Checks whether you have the required permissions for the action, without
	// actually making the request, and provides an error response. If you have the
	// required permissions, the error response is DryRunOperation . Otherwise, it is
	// UnauthorizedOperation .
	DryRun *bool

	// If you’re creating a network interface in a dual-stack or IPv6-only subnet, you
	// have the option to assign a primary IPv6 IP address. A primary IPv6 address is
	// an IPv6 GUA address associated with an ENI that you have enabled to use a
	// primary IPv6 address. Use this option if the instance that this ENI will be
	// attached to relies on its IPv6 address not changing. Amazon Web Services will
	// automatically assign an IPv6 address associated with the ENI attached to your
	// instance to be the primary IPv6 address. Once you enable an IPv6 GUA address to
	// be a primary IPv6, you cannot disable it. When you enable an IPv6 GUA address to
	// be a primary IPv6, the first IPv6 GUA will be made the primary IPv6 address
	// until the instance is terminated or the network interface is detached. If you
	// have multiple IPv6 addresses associated with an ENI attached to your instance
	// and you enable a primary IPv6 address, the first IPv6 GUA address associated
	// with the ENI becomes the primary IPv6 address.
	EnablePrimaryIpv6 *bool

	// The IDs of one or more security groups.
	Groups []string

	// The type of network interface. The default is interface . The only supported
	// values are interface , efa , and trunk .
	InterfaceType types.NetworkInterfaceCreationType

	// The number of IPv4 prefixes that Amazon Web Services automatically assigns to
	// the network interface. You can't specify a count of IPv4 prefixes if you've
	// specified one of the following: specific IPv4 prefixes, specific private IPv4
	// addresses, or a count of private IPv4 addresses.
	Ipv4PrefixCount *int32

	// The IPv4 prefixes assigned to the network interface. You can't specify IPv4
	// prefixes if you've specified one of the following: a count of IPv4 prefixes,
	// specific private IPv4 addresses, or a count of private IPv4 addresses.
	Ipv4Prefixes []types.Ipv4PrefixSpecificationRequest

	// The number of IPv6 addresses to assign to a network interface. Amazon EC2
	// automatically selects the IPv6 addresses from the subnet range. You can't
	// specify a count of IPv6 addresses using this parameter if you've specified one
	// of the following: specific IPv6 addresses, specific IPv6 prefixes, or a count of
	// IPv6 prefixes. If your subnet has the AssignIpv6AddressOnCreation attribute
	// set, you can override that setting by specifying 0 as the IPv6 address count.
	Ipv6AddressCount *int32

	// The IPv6 addresses from the IPv6 CIDR block range of your subnet. You can't
	// specify IPv6 addresses using this parameter if you've specified one of the
	// following: a count of IPv6 addresses, specific IPv6 prefixes, or a count of IPv6
	// prefixes.
	Ipv6Addresses []types.InstanceIpv6Address

	// The number of IPv6 prefixes that Amazon Web Services automatically assigns to
	// the network interface. You can't specify a count of IPv6 prefixes if you've
	// specified one of the following: specific IPv6 prefixes, specific IPv6 addresses,
	// or a count of IPv6 addresses.
	Ipv6PrefixCount *int32

	// The IPv6 prefixes assigned to the network interface. You can't specify IPv6
	// prefixes if you've specified one of the following: a count of IPv6 prefixes,
	// specific IPv6 addresses, or a count of IPv6 addresses.
	Ipv6Prefixes []types.Ipv6PrefixSpecificationRequest

	// The primary private IPv4 address of the network interface. If you don't specify
	// an IPv4 address, Amazon EC2 selects one for you from the subnet's IPv4 CIDR
	// range. If you specify an IP address, you cannot indicate any IP addresses
	// specified in privateIpAddresses as primary (only one IP address can be
	// designated as primary).
	PrivateIpAddress *string

	// The private IPv4 addresses. You can't specify private IPv4 addresses if you've
	// specified one of the following: a count of private IPv4 addresses, specific IPv4
	// prefixes, or a count of IPv4 prefixes.
	PrivateIpAddresses []types.PrivateIpAddressSpecification

	// The number of secondary private IPv4 addresses to assign to a network
	// interface. When you specify a number of secondary IPv4 addresses, Amazon EC2
	// selects these IP addresses within the subnet's IPv4 CIDR range. You can't
	// specify this option and specify more than one private IP address using
	// privateIpAddresses . You can't specify a count of private IPv4 addresses if
	// you've specified one of the following: specific private IPv4 addresses, specific
	// IPv4 prefixes, or a count of IPv4 prefixes.
	SecondaryPrivateIpAddressCount *int32

	// The tags to apply to the new network interface.
	TagSpecifications []types.TagSpecification

	noSmithyDocumentSerde
}

type CreateNetworkInterfaceOutput struct {

	// The token to use to retrieve the next page of results. This value is null when
	// there are no more results to return.
	ClientToken *string

	// Information about the network interface.
	NetworkInterface *types.NetworkInterface

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationCreateNetworkInterfaceMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsEc2query_serializeOpCreateNetworkInterface{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsEc2query_deserializeOpCreateNetworkInterface{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "CreateNetworkInterface"); err != nil {
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
	if err = addIdempotencyToken_opCreateNetworkInterfaceMiddleware(stack, options); err != nil {
		return err
	}
	if err = addOpCreateNetworkInterfaceValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opCreateNetworkInterface(options.Region), middleware.Before); err != nil {
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
	return nil
}

type idempotencyToken_initializeOpCreateNetworkInterface struct {
	tokenProvider IdempotencyTokenProvider
}

func (*idempotencyToken_initializeOpCreateNetworkInterface) ID() string {
	return "OperationIdempotencyTokenAutoFill"
}

func (m *idempotencyToken_initializeOpCreateNetworkInterface) HandleInitialize(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (
	out middleware.InitializeOutput, metadata middleware.Metadata, err error,
) {
	if m.tokenProvider == nil {
		return next.HandleInitialize(ctx, in)
	}

	input, ok := in.Parameters.(*CreateNetworkInterfaceInput)
	if !ok {
		return out, metadata, fmt.Errorf("expected middleware input to be of type *CreateNetworkInterfaceInput ")
	}

	if input.ClientToken == nil {
		t, err := m.tokenProvider.GetIdempotencyToken()
		if err != nil {
			return out, metadata, err
		}
		input.ClientToken = &t
	}
	return next.HandleInitialize(ctx, in)
}
func addIdempotencyToken_opCreateNetworkInterfaceMiddleware(stack *middleware.Stack, cfg Options) error {
	return stack.Initialize.Add(&idempotencyToken_initializeOpCreateNetworkInterface{tokenProvider: cfg.IdempotencyTokenProvider}, middleware.Before)
}

func newServiceMetadataMiddleware_opCreateNetworkInterface(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "CreateNetworkInterface",
	}
}
