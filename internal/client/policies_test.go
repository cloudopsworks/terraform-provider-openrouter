package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPolicyCRUDRequestContract(t *testing.T) {
	for _, kind := range []string{"budget", "model_availability"} {
		t.Run(kind, func(t *testing.T) {
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method)
				path, _ := policyPath(kind)
				if !strings.HasPrefix(r.URL.Path, path) {
					t.Errorf("path=%s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer management" {
					t.Error("missing auth")
				}
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodGet || r.Method == http.MethodDelete {
					if r.URL.Query().Get("workspace_id") != "ws" {
						t.Error("workspace query missing")
					}
				}
				if r.Method == http.MethodPost || r.Method == http.MethodPut {
					var body map[string]json.RawMessage
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					for _, field := range []string{"member_ids", "api_key_hashes", "is_workspace_default", "opted_out_member_ids", "opted_out_api_key_hashes"} {
						if _, ok := body[field]; ok {
							t.Errorf("unexpected assignment/default field %s", field)
						}
					}
					if string(body["workspace_id"]) != `"ws"` {
						t.Error("workspace body missing")
					}
					if _, ok := body["policy_type"]; ok != (r.Method == http.MethodPost) {
						t.Error("policy_type only belongs on create")
					}
					var config map[string]any
					_ = json.Unmarshal(body["config"], &config)
					if kind == "budget" {
						if config["limit_usd"] != nil || config["reset_interval"] != nil || config["include_byok_in_budgets"] != false {
							t.Errorf("budget config=%#v", config)
						}
					} else {
						if config["enforce_zdr_xai"] != false || config["allowed_models"] == nil || config["allowed_providers"] != nil {
							t.Errorf("model config=%#v", config)
						}
					}
				}
				if r.Method == http.MethodDelete {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "policy", "name": "policy name", "policy_type": kind, "config": map[string]any{}}})
			}))
			defer server.Close()
			c := New("management", server.URL, "", time.Second)
			ctx := context.Background()
			var err error
			if kind == "budget" {
				_, err = c.CreateBudgetPolicy(ctx, "ws", "policy name", BudgetPolicyConfig{})
				if err == nil {
					_, err = c.UpdateBudgetPolicy(ctx, "ws", "policy", "policy name", BudgetPolicyConfig{})
				}
			} else {
				f := false
				config := ModelAvailabilityPolicyConfig{AllowedModels: []string{}, EnforceZDRXAI: &f}
				_, err = c.CreateModelAvailabilityPolicy(ctx, "ws", "policy name", config)
				if err == nil {
					_, err = c.UpdateModelAvailabilityPolicy(ctx, "ws", "policy", "policy name", config)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.GetPolicy(ctx, kind, "ws", "policy"); err != nil {
				t.Fatal(err)
			}
			if err = c.DeletePolicy(ctx, kind, "ws", "policy"); err != nil {
				t.Fatal(err)
			}
			if strings.Join(methods, ",") != "POST,PUT,GET,DELETE" {
				t.Fatal(methods)
			}
		})
	}
}

func TestPolicyResponseIdentityAndOwnership(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":{"id":"wrong","name":"name","policy_type":"budget","config":{}}}`, `{"data":{"id":"policy","name":"name","policy_type":"moderation","config":{}}}`, `{"data":{"id":"policy","name":"name","policy_type":"budget","config":null}}`, `{"data":{"id":"policy","name":"name","policy_type":"budget","config":[]}}`, `{"data":{"id":"policy","name":"name","policy_type":"budget","config":{"limit_usd":"invalid"}}}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		c := New("management", server.URL, "", time.Second)
		if _, err := c.GetPolicy(context.Background(), "budget", "ws", "policy"); err == nil {
			t.Errorf("accepted malformed/mismatched response %s", body)
		}
		server.Close()
	}
	for _, policy := range []Policy{{Reference: policyTestString("legacy_mapping")}, {Reference: policyTestString("account_default")}, {IsWorkspaceDefault: true}, {IsAccountPolicy: true}, {ReplacesLegacyDefault: true}} {
		if err := policy.EnsureManageable(); err == nil {
			t.Fatalf("accepted unsafe policy %+v", policy)
		}
	}
	if err := (Policy{}).EnsureManageable(); err != nil {
		t.Fatal(err)
	}
}
func policyTestString(value string) *string { return &value }

func TestPolicyWireShapePrecedesChangedIdentityClassification(t *testing.T) {
	tests := []struct {
		kind, config string
		valid        bool
	}{
		{"budget", `{"include_byok_in_budgets":null}`, false},
		{"budget", `{}`, true},
		{"budget", `{"include_byok_in_budgets":false}`, true},
		{"budget", `{"include_byok_in_budgets":true}`, true},
	}
	for _, field := range []string{"allowed_data_regions", "allowed_models", "allowed_providers", "ignored_models", "ignored_providers", "platform_banned_authors", "platform_banned_models", "platform_banned_providers"} {
		tests = append(tests, struct {
			kind, config string
			valid        bool
		}{"model_availability", `{"` + field + `":[null]}`, false})
		tests = append(tests, struct {
			kind, config string
			valid        bool
		}{"model_availability", `{"` + field + `":["valid",null]}`, false})
		tests = append(tests, struct {
			kind, config string
			valid        bool
		}{"model_availability", `{"` + field + `":null}`, true})
		tests = append(tests, struct {
			kind, config string
			valid        bool
		}{"model_availability", `{"` + field + `":["valid"]}`, true})
	}
	for _, test := range tests {
		t.Run(test.kind+test.config, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("unexpected method %s", r.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": Policy{ID: "changed", Name: "name", PolicyType: test.kind, Config: json.RawMessage(test.config)}})
			}))
			defer server.Close()
			c := New("management", server.URL, "", time.Second)
			var policy *Policy
			var err error
			if test.kind == "budget" {
				policy, err = c.UpdateBudgetPolicy(context.Background(), "ws", "original", "name", BudgetPolicyConfig{})
			} else {
				policy, err = c.UpdateModelAvailabilityPolicy(context.Background(), "ws", "original", "name", ModelAvailabilityPolicyConfig{})
			}
			var changed *PolicyIdentityChangedError
			if test.valid {
				if !errors.As(err, &changed) || policy == nil || policy.ID != "changed" {
					t.Fatalf("valid changed identity not recoverable: policy=%+v err=%v", policy, err)
				}
			} else {
				if err == nil || errors.As(err, &changed) || policy != nil {
					t.Fatalf("malformed config classified as recoverable: policy=%+v err=%v", policy, err)
				}
			}
		})
	}
}

func TestPolicyOwnershipFlagsRejectNull(t *testing.T) {
	for _, field := range []string{"is_workspace_default", "is_account_policy", "replaces_legacy_default"} {
		for _, method := range []string{http.MethodGet, http.MethodPut} {
			t.Run(field+method, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					id := "original"
					if method == http.MethodPut {
						id = "unvalidated-copy"
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id, "name": "name", "policy_type": "budget", "config": map[string]any{}, field: nil}})
				}))
				defer server.Close()
				c := New("management", server.URL, "", time.Second)
				var policy *Policy
				var err error
				if method == http.MethodGet {
					policy, err = c.GetPolicy(context.Background(), "budget", "ws", "original")
				} else {
					policy, err = c.UpdateBudgetPolicy(context.Background(), "ws", "original", "name", BudgetPolicyConfig{})
				}
				var changed *PolicyIdentityChangedError
				if err == nil || errors.As(err, &changed) || policy != nil {
					t.Fatalf("null ownership flag authorized recovery or mutation: policy=%+v err=%v", policy, err)
				}
			})
		}
	}
}
