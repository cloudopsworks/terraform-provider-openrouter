package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPolicyAssignmentsClient(t *testing.T) {
	for _, subjectType := range []string{"api-keys", "members"} {
		t.Run(subjectType, func(t *testing.T) {
			expected := PolicyAssignmentState{PolicyIDs: []string{}, WorkspaceDefaultPolicyIDs: []string{"inherited"}, OptedOut: false}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/guardrails/policies/assignments/"+subjectType+"/subject" {
					t.Errorf("path=%s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer token" {
					t.Error("missing management token")
				}
				if r.Method == http.MethodGet {
					if r.URL.Query().Get("workspace_id") != "workspace" || r.URL.Query().Get("policy_type") != "budget" {
						t.Errorf("query=%s", r.URL.RawQuery)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": expected})
					return
				}
				var body ChangePolicyAssignmentRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.WorkspaceID != "workspace" || body.PolicyType != "budget" || body.PolicyID != "policy" || !body.Assigned || !reflect.DeepEqual(body.ExpectedAssignment, expected) {
					t.Errorf("body=%#v", body)
				}
				if r.URL.RawQuery != "" {
					t.Errorf("unexpected PUT query=%s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`{"data":true}`))
			}))
			defer server.Close()
			c := New("token", server.URL, "", time.Second)
			current, err := c.GetPolicyAssignment(context.Background(), subjectType, "subject", "workspace", "budget")
			if err != nil {
				t.Fatal(err)
			}
			err = c.ChangePolicyAssignment(context.Background(), subjectType, "subject", ChangePolicyAssignmentRequest{WorkspaceID: "workspace", PolicyType: "budget", PolicyID: "policy", Assigned: true, ExpectedAssignment: *current})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}
func TestPolicyAssignmentResponseValidation(t *testing.T) {
	for _, payload := range []string{`{}`, `{"data":null}`, `{"data":{}}`, `{"data":{"policy_ids":[],"workspace_default_policy_ids":[]}}`, `{"data":{"policy_ids":null,"workspace_default_policy_ids":[],"opted_out":false}}`, `{"data":{"policy_ids":[""],"workspace_default_policy_ids":[],"opted_out":false}}`, `{"data":{"policy_ids":["p","p"],"workspace_default_policy_ids":[],"opted_out":false}}`, `{"data":{"policy_ids":["p","other"],"workspace_default_policy_ids":[],"opted_out":false}}`} {
		t.Run(payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(payload)) }))
			defer server.Close()
			if _, err := New("token", server.URL, "", time.Second).GetPolicyAssignment(context.Background(), "members", "user", "ws", "budget"); err == nil {
				t.Fatal("expected malformed concurrency token error")
			}
		})
	}
}
func TestPolicyAssignmentConflictNotRetried(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, `{"error":{"message":"concurrent change"}}`, http.StatusConflict)
	}))
	defer server.Close()
	err := New("token", server.URL, "", time.Second).ChangePolicyAssignment(context.Background(), "members", "user", ChangePolicyAssignmentRequest{ExpectedAssignment: PolicyAssignmentState{PolicyIDs: []string{}, WorkspaceDefaultPolicyIDs: []string{}}})
	if err == nil || !strings.Contains(err.Error(), "(409)") || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
func TestDirectAssignmentPolicySafety(t *testing.T) {
	for _, test := range []struct {
		payload string
		valid   bool
	}{
		{`{"data":{"id":"p","policy_type":"model_availability"}}`, true},
		{`{"data":{"id":"other","policy_type":"model_availability"}}`, false},
		{`{"data":{"id":"p","policy_type":"budget"}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","reference":"legacy_mapping"}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","reference":"account_default"}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","is_workspace_default":true}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","is_workspace_default":null}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","is_account_policy":null}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","replaces_legacy_default":null}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","is_workspace_default":false,"is_account_policy":false,"replaces_legacy_default":false}}`, true},
		{`{"data":{"id":"p","policy_type":"model_availability","is_account_policy":true}}`, false},
		{`{"data":{"id":"p","policy_type":"model_availability","replaces_legacy_default":true}}`, false},
	} {
		t.Run(test.payload, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/guardrails/policies/model-availability/p" || r.URL.Query().Get("workspace_id") != "ws" {
					t.Errorf("request=%s %s", r.Method, r.URL)
				}
				_, _ = w.Write([]byte(test.payload))
			}))
			defer server.Close()
			err := New("token", server.URL, "", time.Second).ValidateDirectAssignmentPolicy(context.Background(), "ws", "model_availability", "p")
			if (err == nil) != test.valid {
				t.Fatalf("err=%v valid=%v", err, test.valid)
			}
		})
	}
}
