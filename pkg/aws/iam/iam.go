// Package iam implements various IAM components.
package iam

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/iam/iamiface"
	"go.uber.org/zap"
)

// PolicyDocument is the IAM policy document.
type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

// StatementEntry is the entry in IAM policy document "Statement" field.
type StatementEntry struct {
	Effect    string          `json:"Effect,omitempty"`
	Action    []string        `json:"Action,omitempty"`
	Resource  string          `json:"Resource,omitempty"`
	Principal *PrincipalEntry `json:"Principal,omitempty"`
}

// PrincipalEntry represents the policy document Principal.
type PrincipalEntry struct {
	Service []string `json:"Service,omitempty"`
}

type AssumeRolePolicyDocument struct {
	Version   string                               `json:"Version"`
	Statement []*AssumeRolePolicyDocumentStatement `json:"Statement"`
}

type AssumeRolePolicyDocumentStatement struct {
	Effect    string          `json:"Effect"`
	Principal *PrincipalEntry `json:"Principal,omitempty"`
}

// Validate validates IAM role.
func Validate(
	lg *zap.Logger,
	iamAPI iamiface.IAMAPI,
	roleName string,
	requiredSPs []string,
	requiredPolicyARNs []string,
) error {
	lg.Info("validating role service principals",
		zap.String("role-name", roleName),
	)
	out, err := iamAPI.GetRole(&iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		lg.Warn("failed to GetRole", zap.Error(err))
		return err
	}
	txt := aws.StringValue(out.Role.AssumeRolePolicyDocument)
	txt, err = url.QueryUnescape(txt)
	if err != nil {
		return fmt.Errorf("failed to escape AssumeRolePolicyDocument:\n%s\n\n(%v)", txt, err)
	}
	doc := new(AssumeRolePolicyDocument)
	if err = json.Unmarshal([]byte(txt), doc); err != nil {
		return fmt.Errorf("failed to unmarshal AssumeRolePolicyDocument:\n%s\n\n(%v)", txt, err)
	}
	trustedEntities := make(map[string]struct{})
	for _, v1 := range doc.Statement {
		for _, v2 := range v1.Principal.Service {
			lg.Info("found trusted entity", zap.String("entity", v2))
			trustedEntities[v2] = struct{}{}
		}
	}
	reqEnts := make(map[string]struct{})
	for _, v := range requiredSPs {
		reqEnts[v] = struct{}{}
	}
	for k := range reqEnts {
		if _, ok := trustedEntities[k]; !ok {
			return fmt.Errorf("Principal.Service missing %q", k)
		}
	}

	lg.Info("validating role policies", zap.String("role-name", roleName))
	lout, err := iamAPI.ListAttachedRolePolicies(&iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	if err != nil {
		lg.Warn("failed to ListAttachedRolePolicies", zap.Error(err))
		return err
	}
	attached := make(map[string]struct{})
	for _, p := range lout.AttachedPolicies {
		arn := aws.StringValue(p.PolicyArn)
		lg.Info("found attached policy ARN", zap.String("policy-arn", arn))
		attached[arn] = struct{}{}
	}
	reqPols := make(map[string]struct{})
	for _, v := range requiredPolicyARNs {
		reqPols[v] = struct{}{}
	}
	for k := range reqPols {
		if _, ok := attached[k]; !ok {
			return fmt.Errorf("PolicyARNs missing %q", k)
		}
	}
	return nil
}
