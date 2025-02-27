package eksapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type VPCTestCase struct {
	name          string
	numSubnets    int
	expectedCIDRs []string
	wantErr       bool // Optional, for error checking
}

func Test_getValidSubnetCIDRs(t *testing.T) {
	testCases := []struct {
		name          string
		numSubnets    int
		expectedCIDRs []string
		wantErr       bool // Optional, for error checking
	}{
		{
			// more subnets than supported (256)
			numSubnets: 257,
			wantErr:    true,
		},
		{
			// single subnet claims all IPs
			numSubnets:    1,
			expectedCIDRs: []string{"192.168.0.0/16"},
		},
		{
			numSubnets:    2,
			expectedCIDRs: []string{"192.168.0.0/17", "192.168.128.0/17"},
		},
		{
			numSubnets:    4,
			expectedCIDRs: []string{"192.168.0.0/18", "192.168.64.0/18", "192.168.128.0/18", "192.168.192.0/18"},
		},
		{
			// each subnet takes a 1 of the 256 available values in the third octet
			numSubnets:    256,
			expectedCIDRs: []string{"192.168.0.0/24", "192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24", "192.168.4.0/24", "192.168.5.0/24", "192.168.6.0/24", "192.168.7.0/24", "192.168.8.0/24", "192.168.9.0/24", "192.168.10.0/24", "192.168.11.0/24", "192.168.12.0/24", "192.168.13.0/24", "192.168.14.0/24", "192.168.15.0/24", "192.168.16.0/24", "192.168.17.0/24", "192.168.18.0/24", "192.168.19.0/24", "192.168.20.0/24", "192.168.21.0/24", "192.168.22.0/24", "192.168.23.0/24", "192.168.24.0/24", "192.168.25.0/24", "192.168.26.0/24", "192.168.27.0/24", "192.168.28.0/24", "192.168.29.0/24", "192.168.30.0/24", "192.168.31.0/24", "192.168.32.0/24", "192.168.33.0/24", "192.168.34.0/24", "192.168.35.0/24", "192.168.36.0/24", "192.168.37.0/24", "192.168.38.0/24", "192.168.39.0/24", "192.168.40.0/24", "192.168.41.0/24", "192.168.42.0/24", "192.168.43.0/24", "192.168.44.0/24", "192.168.45.0/24", "192.168.46.0/24", "192.168.47.0/24", "192.168.48.0/24", "192.168.49.0/24", "192.168.50.0/24", "192.168.51.0/24", "192.168.52.0/24", "192.168.53.0/24", "192.168.54.0/24", "192.168.55.0/24", "192.168.56.0/24", "192.168.57.0/24", "192.168.58.0/24", "192.168.59.0/24", "192.168.60.0/24", "192.168.61.0/24", "192.168.62.0/24", "192.168.63.0/24", "192.168.64.0/24", "192.168.65.0/24", "192.168.66.0/24", "192.168.67.0/24", "192.168.68.0/24", "192.168.69.0/24", "192.168.70.0/24", "192.168.71.0/24", "192.168.72.0/24", "192.168.73.0/24", "192.168.74.0/24", "192.168.75.0/24", "192.168.76.0/24", "192.168.77.0/24", "192.168.78.0/24", "192.168.79.0/24", "192.168.80.0/24", "192.168.81.0/24", "192.168.82.0/24", "192.168.83.0/24", "192.168.84.0/24", "192.168.85.0/24", "192.168.86.0/24", "192.168.87.0/24", "192.168.88.0/24", "192.168.89.0/24", "192.168.90.0/24", "192.168.91.0/24", "192.168.92.0/24", "192.168.93.0/24", "192.168.94.0/24", "192.168.95.0/24", "192.168.96.0/24", "192.168.97.0/24", "192.168.98.0/24", "192.168.99.0/24", "192.168.100.0/24", "192.168.101.0/24", "192.168.102.0/24", "192.168.103.0/24", "192.168.104.0/24", "192.168.105.0/24", "192.168.106.0/24", "192.168.107.0/24", "192.168.108.0/24", "192.168.109.0/24", "192.168.110.0/24", "192.168.111.0/24", "192.168.112.0/24", "192.168.113.0/24", "192.168.114.0/24", "192.168.115.0/24", "192.168.116.0/24", "192.168.117.0/24", "192.168.118.0/24", "192.168.119.0/24", "192.168.120.0/24", "192.168.121.0/24", "192.168.122.0/24", "192.168.123.0/24", "192.168.124.0/24", "192.168.125.0/24", "192.168.126.0/24", "192.168.127.0/24", "192.168.128.0/24", "192.168.129.0/24", "192.168.130.0/24", "192.168.131.0/24", "192.168.132.0/24", "192.168.133.0/24", "192.168.134.0/24", "192.168.135.0/24", "192.168.136.0/24", "192.168.137.0/24", "192.168.138.0/24", "192.168.139.0/24", "192.168.140.0/24", "192.168.141.0/24", "192.168.142.0/24", "192.168.143.0/24", "192.168.144.0/24", "192.168.145.0/24", "192.168.146.0/24", "192.168.147.0/24", "192.168.148.0/24", "192.168.149.0/24", "192.168.150.0/24", "192.168.151.0/24", "192.168.152.0/24", "192.168.153.0/24", "192.168.154.0/24", "192.168.155.0/24", "192.168.156.0/24", "192.168.157.0/24", "192.168.158.0/24", "192.168.159.0/24", "192.168.160.0/24", "192.168.161.0/24", "192.168.162.0/24", "192.168.163.0/24", "192.168.164.0/24", "192.168.165.0/24", "192.168.166.0/24", "192.168.167.0/24", "192.168.168.0/24", "192.168.169.0/24", "192.168.170.0/24", "192.168.171.0/24", "192.168.172.0/24", "192.168.173.0/24", "192.168.174.0/24", "192.168.175.0/24", "192.168.176.0/24", "192.168.177.0/24", "192.168.178.0/24", "192.168.179.0/24", "192.168.180.0/24", "192.168.181.0/24", "192.168.182.0/24", "192.168.183.0/24", "192.168.184.0/24", "192.168.185.0/24", "192.168.186.0/24", "192.168.187.0/24", "192.168.188.0/24", "192.168.189.0/24", "192.168.190.0/24", "192.168.191.0/24", "192.168.192.0/24", "192.168.193.0/24", "192.168.194.0/24", "192.168.195.0/24", "192.168.196.0/24", "192.168.197.0/24", "192.168.198.0/24", "192.168.199.0/24", "192.168.200.0/24", "192.168.201.0/24", "192.168.202.0/24", "192.168.203.0/24", "192.168.204.0/24", "192.168.205.0/24", "192.168.206.0/24", "192.168.207.0/24", "192.168.208.0/24", "192.168.209.0/24", "192.168.210.0/24", "192.168.211.0/24", "192.168.212.0/24", "192.168.213.0/24", "192.168.214.0/24", "192.168.215.0/24", "192.168.216.0/24", "192.168.217.0/24", "192.168.218.0/24", "192.168.219.0/24", "192.168.220.0/24", "192.168.221.0/24", "192.168.222.0/24", "192.168.223.0/24", "192.168.224.0/24", "192.168.225.0/24", "192.168.226.0/24", "192.168.227.0/24", "192.168.228.0/24", "192.168.229.0/24", "192.168.230.0/24", "192.168.231.0/24", "192.168.232.0/24", "192.168.233.0/24", "192.168.234.0/24", "192.168.235.0/24", "192.168.236.0/24", "192.168.237.0/24", "192.168.238.0/24", "192.168.239.0/24", "192.168.240.0/24", "192.168.241.0/24", "192.168.242.0/24", "192.168.243.0/24", "192.168.244.0/24", "192.168.245.0/24", "192.168.246.0/24", "192.168.247.0/24", "192.168.248.0/24", "192.168.249.0/24", "192.168.250.0/24", "192.168.251.0/24", "192.168.252.0/24", "192.168.253.0/24", "192.168.254.0/24", "192.168.255.0/24"},
		},
	}
	for _, tc := range testCases {
		subnetCidrs, err := getValidSubnetCIDRs(tc.numSubnets)
		if !tc.wantErr {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
		assert.Equal(t, tc.expectedCIDRs, subnetCidrs)
	}
}
