// Package iam implements various IAM components.
package iam

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
