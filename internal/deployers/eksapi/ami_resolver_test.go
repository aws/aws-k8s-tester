//go:build integration

package eksapi

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
)

func TestAMIResolver(t *testing.T) {
	ctx := context.Background()
	awsCfg, err := config.LoadDefaultConfig(ctx)
	assert.NoError(t, err)

	amiResolver := NewAMIResolver(newAWSClients(awsCfg, ""))

	t.Run("AL2023-nvidia", func(t *testing.T) {
		opts := deployerOptions{
			UserDataFormat:    UserDataNodeadm,
			KubernetesVersion: "1.33",
		}
		t.Run("nvidia", func(t *testing.T) {
			opts := opts
			opts.InstanceTypes = []string{"g5.xlarge"}

			ami, err := amiResolver.Resolve(ctx, &opts)
			assert.NoError(t, err)
			assert.Regexp(t, "ami-.*", ami)
		})
		t.Run("standard", func(t *testing.T) {
			opts := opts
			opts.InstanceTypes = []string{"m5.xlarge"}

			ami, err := amiResolver.Resolve(ctx, &opts)
			assert.NoError(t, err)
			assert.Regexp(t, "ami-.*", ami)
		})
	})

	t.Run("Bottlerocket", func(t *testing.T) {
		opts := deployerOptions{
			UserDataFormat:    UserDataBottlerocket,
			KubernetesVersion: "1.33",
		}
		t.Run("nvidia", func(t *testing.T) {
			opts := opts
			opts.InstanceTypes = []string{"g5.xlarge"}

			ami, err := amiResolver.Resolve(ctx, &opts)
			assert.NoError(t, err)
			assert.Regexp(t, "ami-.*", ami)
		})
		t.Run("standard", func(t *testing.T) {
			opts := opts
			opts.InstanceTypes = []string{"m5.xlarge"}

			ami, err := amiResolver.Resolve(ctx, &opts)
			assert.NoError(t, err)
			assert.Regexp(t, "ami-.*", ami)
		})
	})
}
