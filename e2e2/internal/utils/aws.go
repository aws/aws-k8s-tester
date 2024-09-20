package utils

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// NewConfig returns an AWS SDK config
// It will panic if the cnfig cannot be created
func NewConfig() (aws.Config, error) {
	c, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to create AWS SDK config: %v", err)
	}
	return c, nil
}

// creating the payloadHash
func CreatePayloadHash(req *http.Request) (string, error) {
	var payloadHash string
	if req.Body == nil { // For a GET Request (no payload)
		payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	} else { // For every other request
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(req.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read request body %q", err)
		}
		req.Body = io.NopCloser(buf)
		hash := sha256.Sum256(buf.Bytes())
		payloadHash = hex.EncodeToString(hash[:])
	}
	return payloadHash, nil
}
