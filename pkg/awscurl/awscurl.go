package awscurl

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
)

type client struct {
	cfg Config
	http1Client *http.Client

	payload []byte
}

func New(cfg Config) *client {
	return &client{
		cfg: cfg,
		http1Client: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 5 * time.Second,
				}).DialContext,
			},
		},
	}
}

func (c *client) Do() (res string, err error) {
	var req *http.Request
	var resp *http.Response
	var respData []byte

	req, err = c.signRequest()
	if err != nil {
		return "", err
	}
	resp, err = c.http1Client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.Body == nil {
		return "", fmt.Errorf("fail get nil resp Body")
	}
	defer resp.Body.Close()

	respData, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(respData), nil
}

func (c *client) signRequest() (*http.Request, error) {
	req, err := c.makeRequest(c.cfg.Method)
	if err != nil {
		return nil, err
	}
	bodySHA256 := sha256Hash(c.payload)

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(c.cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config, %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5 *time.Second)
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, err
	}
	cancel()

	signer := v4.NewSigner()
	err = signer.SignHTTP(req.Context(), creds, req, bodySHA256, c.cfg.Service, c.cfg.Region, time.Now())
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (c *client) makeRequest(method string) (req *http.Request, err error) {
	c.payload, err = c.preparePayload()
	if err != nil {
		return nil, err
	}

	body := bytes.NewReader(c.payload)
	req, err = http.NewRequest(method, c.cfg.URI, body)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (c *client) preparePayload() (payloadData []byte, err error) {
	customFlagsConfig := customFlagsConfig{
		ControllerManager: qpsBurst{
			KubeApiQps: c.cfg.KubeControllerManagerQPS,
			KubeApiBurst: c.cfg.KubeControllerManagerBurst,
		},
		Scheduler:         qpsBurst{
			KubeApiQps: c.cfg.KubeSchedulerQPS,
			KubeApiBurst: c.cfg.KubeSchedulerBurst,
		},
	}
	var customFlagsConfigData []byte
	customFlagsConfigData, err = json.Marshal(customFlagsConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to Marshal customFlagsConfig %v, %v", customFlagsConfig, err)
	}

	payload := payload{
		ClusterArn: c.cfg.ClusterArn,
		CustomFlagsConfig: string(customFlagsConfigData),
	}
	payloadData, err = json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("unable to Marshal payload %v, %v", payload, err)
	}

	return payloadData, nil
}

func sha256Hash(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}