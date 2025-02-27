package eksapi

import (
	"fmt"
	"math"
)

const (
	baseVPCIPFormat = "192.168.%d.0"
	subnetBits      = 8
	baseNetMask     = 16
)

var (
	vpcCidrIP    = fmt.Sprintf(baseVPCIPFormat, 0)
	vpcCidrBlock = fmt.Sprintf("%s/%d", vpcCidrIP, baseNetMask)
)

type vpcConfig struct {
	VPCCIDRBlock string
	PublicCIDRs  []string
	PrivateCIDRs []string

	// AZs only used for getting AZ by index of a subnet CIDR, doesn't have to be unique
	AvailabilityZones []string
	Ipv6CidrBits      int
	NumSubnets        int
}

func getNumSubnetBits(numSubnets int) int {
	return int(math.Ceil(math.Log2(float64(numSubnets))))
}

func getValidSubnetCIDRs(numSubnets int) (subnets []string, err error) {
	if numSubnets > 256 {
		// logic gets significantly more messy here and also
		// IP range gets extremely restrictive
		// error only here for visibility, will probably never
		// be needed
		return nil, fmt.Errorf("cannot create more than 256 subnets")
	}
	numNetBitsNeeded := getNumSubnetBits(numSubnets)
	subnetNetMask := baseNetMask + numNetBitsNeeded
	shift := subnetBits - numNetBitsNeeded
	for subnetId := range numSubnets {
		subnetBits := subnetId << shift
		subnetCidrIP := fmt.Sprintf(baseVPCIPFormat, subnetBits)
		subnets = append(subnets, fmt.Sprintf("%s/%d", subnetCidrIP, subnetNetMask))
	}
	return subnets, nil
}

func getVpcConfig(availabilityZones []string) (*vpcConfig, error) {
	numAZs := len(availabilityZones)
	// keep it simple so the template can get the AZ by index of the CIDR
	numSubnets := numAZs * 2 // 1 private, 1 public per AZ
	subnetCidrs, err := getValidSubnetCIDRs(numSubnets)
	if err != nil {
		return nil, err
	}
	return &vpcConfig{
		VPCCIDRBlock:      vpcCidrBlock,
		PublicCIDRs:       subnetCidrs[:numSubnets/2], // use first half as public
		PrivateCIDRs:      subnetCidrs[numSubnets/2:], // use second half as private
		AvailabilityZones: availabilityZones,
		NumSubnets:        numSubnets,
		// https://docs.aws.amazon.com/vpc/latest/userguide/vpc-cidr-blocks.html#vpc-sizing-ipv6
		Ipv6CidrBits: 64, // leaves 12 subnet bits, ipv4 is the bottleneck
	}, nil
}
