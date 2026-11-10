package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// PolicyAssignmentState is the complete concurrency token returned by OpenRouter.
// Required response fields are validated so a partial response can never authorize a write.
type PolicyAssignmentState struct {
	PolicyIDs                 []string `json:"policy_ids"`
	WorkspaceDefaultPolicyIDs []string `json:"workspace_default_policy_ids"`
	OptedOut                  bool     `json:"opted_out"`
}

func (s *PolicyAssignmentState) UnmarshalJSON(data []byte) error {
	var wire struct {
		PolicyIDs                 *[]string `json:"policy_ids"`
		WorkspaceDefaultPolicyIDs *[]string `json:"workspace_default_policy_ids"`
		OptedOut                  *bool     `json:"opted_out"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.PolicyIDs == nil || wire.WorkspaceDefaultPolicyIDs == nil || wire.OptedOut == nil {
		return fmt.Errorf("openrouter policy assignment response missing required concurrency state")
	}
	*s = PolicyAssignmentState{PolicyIDs: *wire.PolicyIDs, WorkspaceDefaultPolicyIDs: *wire.WorkspaceDefaultPolicyIDs, OptedOut: *wire.OptedOut}
	return s.Validate()
}

func (s PolicyAssignmentState) Validate() error {
	if len(s.PolicyIDs) > 1 {
		return fmt.Errorf("policy assignment response contains multiple direct policies; refusing ambiguous ownership")
	}
	if s.PolicyIDs == nil || s.WorkspaceDefaultPolicyIDs == nil {
		return fmt.Errorf("policy assignment concurrency lists must not be null")
	}
	for _, ids := range [][]string{s.PolicyIDs, s.WorkspaceDefaultPolicyIDs} {
		seen := map[string]bool{}
		for _, id := range ids {
			if strings.TrimSpace(id) == "" || seen[id] {
				return fmt.Errorf("policy assignment response contains empty or duplicate policy IDs")
			}
			seen[id] = true
		}
	}
	return nil
}

type ChangePolicyAssignmentRequest struct {
	WorkspaceID        string                `json:"workspace_id"`
	PolicyType         string                `json:"policy_type"`
	PolicyID           string                `json:"policy_id"`
	Assigned           bool                  `json:"assigned"`
	ExpectedAssignment PolicyAssignmentState `json:"expected_assignment"`
}

func policyAssignmentPath(subjectType, subjectID string) (string, error) {
	if subjectType != "api-keys" && subjectType != "members" {
		return "", fmt.Errorf("unsupported policy assignment subject type")
	}
	if strings.TrimSpace(subjectID) == "" {
		return "", fmt.Errorf("policy assignment subject must not be empty")
	}
	return "/guardrails/policies/assignments/" + subjectType + "/" + url.PathEscape(subjectID), nil
}

func (c *Client) GetPolicyAssignment(ctx context.Context, subjectType, subjectID, workspaceID, policyType string) (*PolicyAssignmentState, error) {
	endpoint, err := policyAssignmentPath(subjectType, subjectID)
	if err != nil {
		return nil, err
	}
	query := url.Values{"workspace_id": {workspaceID}, "policy_type": {policyType}}
	var resp singleResponse[*PolicyAssignmentState]
	if err := c.do(ctx, http.MethodGet, endpoint, query, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("openrouter policy assignment response missing data")
	}
	return resp.Data, nil
}

func (c *Client) ChangePolicyAssignment(ctx context.Context, subjectType, subjectID string, body ChangePolicyAssignmentRequest) error {
	endpoint, err := policyAssignmentPath(subjectType, subjectID)
	if err != nil {
		return err
	}
	if err := body.ExpectedAssignment.Validate(); err != nil {
		return err
	}
	var resp struct {
		Data *bool `json:"data"`
	}
	if err := c.do(ctx, http.MethodPut, endpoint, nil, body, &resp); err != nil {
		return err
	}
	if resp.Data == nil {
		return fmt.Errorf("openrouter policy assignment change response missing data")
	}
	return nil
}

// ValidateDirectAssignmentPolicy rejects policies with inherited, account-owned,
// or legacy migration semantics. This resource manages only ordinary direct bindings.
func (c *Client) ValidateDirectAssignmentPolicy(ctx context.Context, workspaceID, policyType, policyID string) error {
	var kind string
	switch policyType {
	case "budget", "moderation":
		kind = policyType
	case "model_availability":
		kind = "model-availability"
	default:
		return fmt.Errorf("unsupported direct policy type %q", policyType)
	}
	var resp singleResponse[Policy]
	if err := c.do(ctx, http.MethodGet, "/guardrails/policies/"+kind+"/"+url.PathEscape(policyID), url.Values{"workspace_id": {workspaceID}}, nil, &resp); err != nil {
		return err
	}
	p := resp.Data
	if p.ID != policyID || p.PolicyType != policyType {
		return fmt.Errorf("openrouter returned a different policy identity or type")
	}
	return p.EnsureManageable()
}
