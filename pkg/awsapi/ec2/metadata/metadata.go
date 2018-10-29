package metadata

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

/*
curl -L http://169.254.169.254/latest/meta-data

ami-id
ami-launch-index
ami-manifest-path
block-device-mapping/
events/
hostname
instance-action
instance-id
instance-type
local-hostname
local-ipv4
mac
metrics/
network/
placement/
profile
public-hostname
public-ipv4
public-keys/
reservation-id
security-groups
services/
*/

func getMeta(lg *zap.Logger, key string) (s string, err error) {
	addr := fmt.Sprintf("http://169.254.169.254/latest/meta-data/%s", key)
	for i := 0; i < 10; i++ {
		var resp *http.Response
		resp, err = http.Get(addr)
		if err != nil {
			lg.Warn("failed to get public IPv4", zap.Error(err))
			time.Sleep(3 * time.Second)
			continue
		}

		var d []byte
		d, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			lg.Warn("failed to get public IPv4", zap.Error(err))
			time.Sleep(3 * time.Second)
			continue
		}
		resp.Body.Close()

		s = strings.TrimSpace(string(d))
		break
	}
	return s, nil
}

// PublicIPv4 returns the public IPv4 address of an EC2 instance.
func PublicIPv4(lg *zap.Logger) (s string, err error) {
	return getMeta(lg, "public-ipv4")
}

// PrivateIPv4 returns the private IPv4 address of an EC2 instance.
func PrivateIPv4(lg *zap.Logger) (s string, err error) {
	return getMeta(lg, "local-ipv4")
}

// InstanceID returns the instance ID of an EC2 instance.
func InstanceID(lg *zap.Logger) (s string, err error) {
	return getMeta(lg, "instance-id")
}
