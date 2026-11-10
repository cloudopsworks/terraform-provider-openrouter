package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testSCIMMapping(id, group, workspace, role string) SCIMGroupMapping {
	return SCIMGroupMapping{ID: id, OrganizationID: "org", SCIMGroupID: group, WorkspaceID: workspace, Role: role, CreatedAt: "2026-11-01", UpdatedAt: "2026-11-01"}
}
func TestSCIMMappingClientLifecycle(t *testing.T) {
	current := testSCIMMapping("mapping", "group", "workspace", "member")
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/scim/group-mappings" && r.Method == http.MethodGet {
			if r.URL.Query().Get("offset") != "0" || r.URL.Query().Get("limit") != "100" {
				t.Errorf("query %s", r.URL.RawQuery)
			}
			json.NewEncoder(w).Encode(map[string]any{"data": []SCIMGroupMapping{current}, "total_count": 1})
			return
		}
		switch r.Method {
		case http.MethodPost:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if len(body) != 3 || body["scim_group_id"] != "group" || body["workspace_id"] != "workspace" || body["role"] != "member" {
				t.Errorf("body %v", body)
			}
		case http.MethodPatch:
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if len(body) != 1 || body["role"] != "admin" {
				t.Errorf("body %v", body)
			}
			current.Role = "admin"
		case http.MethodDelete:
			if r.URL.Query().Get("keep_members") != "true" {
				t.Errorf("missing safe policy: %s", r.URL.RawQuery)
			}
			fmt.Fprint(w, `{"deleted":true}`)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": current})
	}))
	defer server.Close()
	c := New("local", server.URL, "", time.Second)
	ctx := context.Background()
	if _, err := c.ListSCIMGroupMappings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSCIMGroupMapping(ctx, SCIMGroupMappingCreateRequest{"group", "workspace", "member"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetSCIMGroupMapping(ctx, "mapping"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UpdateSCIMGroupMapping(ctx, "mapping", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSCIMGroupMapping(ctx, "mapping", true); err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "GET,POST,GET,PATCH,DELETE" {
		t.Fatal(methods)
	}
}
func TestSCIMMappingClientRejectsMalformedIdentity(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{"wrong-id", `{"data":{"id":"other","organization_id":"org","scim_group_id":"group","workspace_id":"workspace","role":"member","created_at":"now","updated_at":"now"}}`},
		{"missing-data", `{}`}, {"null-id", `{"data":{"id":null}}`}, {"null-role", `{"data":{"id":"mapping","organization_id":"org","scim_group_id":"group","workspace_id":"workspace","role":null}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, test.body) }))
			defer server.Close()
			if _, err := New("k", server.URL, "", time.Second).GetSCIMGroupMapping(context.Background(), "mapping"); err == nil {
				t.Fatal("accepted malformed identity")
			}
		})
	}
}
func TestSCIMMappingClientDeleteExplicitPolicy(t *testing.T) {
	for _, keep := range []bool{true, false} {
		t.Run(fmt.Sprint(keep), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" || r.URL.Query().Get("keep_members") != fmt.Sprint(keep) {
					t.Errorf("%s %s", r.Method, r.URL)
				}
				fmt.Fprint(w, `{"deleted":true}`)
			}))
			defer server.Close()
			if err := New("k", server.URL, "", time.Second).DeleteSCIMGroupMapping(context.Background(), "mapping", keep); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestSCIMGroupsPaginationAndFilters(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("display_name") != "Engineering" || r.URL.Query().Get("external_id") != "external" {
			t.Errorf("filters %s", r.URL.RawQuery)
		}
		id := "z"
		if r.URL.Query().Get("offset") == "1" {
			id = "a"
		} else if r.URL.Query().Get("offset") != "0" {
			t.Errorf("offset %s", r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []SCIMGroup{{ID: id, OrganizationID: "org", ExternalID: func() *string { v := "external"; return &v }(), DisplayName: "Engineering", CreatedAt: "now", UpdatedAt: "now"}}, "total_count": 2})
	}))
	defer server.Close()
	groups, err := New("k", server.URL, "", time.Second).ListSCIMGroups(context.Background(), "Engineering", "external")
	if err != nil || len(groups) != 2 || groups[0].ID != "a" || calls != 2 {
		t.Fatalf("groups=%v err=%v calls=%d", groups, err, calls)
	}
}
func TestSCIMListRejectsMalformedEnvelope(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":null,"total_count":0}`, `{"data":[],"total_count":null}`, `{"data":[],"total_count":2}`, `{"data":[],"total_count":-1}`, `{"data":[],"total_count":100001}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			if _, err := New("k", server.URL, "", time.Second).ListSCIMGroupMappings(context.Background()); err == nil {
				t.Fatal("accepted malformed list")
			}
		})
	}
}
func TestSCIMListRejectsDuplicateOrChangingPages(t *testing.T) {
	for _, changeTotal := range []bool{false, true} {
		t.Run(fmt.Sprint(changeTotal), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				total := 2
				if changeTotal && calls > 1 {
					total = 3
				}
				json.NewEncoder(w).Encode(map[string]any{"data": []SCIMGroupMapping{testSCIMMapping("same", "g", "w", "member")}, "total_count": total})
			}))
			defer server.Close()
			if _, err := New("k", server.URL, "", time.Second).ListSCIMGroupMappings(context.Background()); err == nil {
				t.Fatal("accepted inconsistent pagination")
			}
			if calls != 2 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}
func TestSCIMClientPropagatesHTTPFailures(t *testing.T) {
	for _, status := range []int{401, 403, 404, 409, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"message":"blocked"}}`)
			}))
			defer server.Close()
			_, err := New("k", server.URL, "", time.Second).CreateSCIMGroupMapping(context.Background(), SCIMGroupMappingCreateRequest{"g", "w", "member"})
			if err == nil || !strings.HasPrefix(err.Error(), fmt.Sprintf("openrouter API error (%d)", status)) || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
		})
	}
}

func TestSCIMMappingCreateMalformedSuccessProvidesOnlyMatchingRecoveryCandidate(t *testing.T) {
	const candidateID = "770e8400-e29b-41d4-a716-446655440000"
	const undisclosed = "private-response-marker"
	for _, test := range []struct {
		name          string
		data          string
		wantCandidate bool
	}{
		{"missing-metadata", `{"id":"` + candidateID + `","scim_group_id":"group","workspace_id":"workspace","unknown":"` + undisclosed + `"}`, true},
		{"malformed-metadata", `{"id":"` + candidateID + `","scim_group_id":"group","workspace_id":"workspace","organization_id":{"unknown":"` + undisclosed + `"},"created_at":7}`, true},
		{"wrong-group", `{"id":"` + candidateID + `","scim_group_id":"another-group","workspace_id":"workspace","unknown":"` + undisclosed + `"}`, false},
		{"wrong-workspace", `{"id":"` + candidateID + `","scim_group_id":"group","workspace_id":"another-workspace","unknown":"` + undisclosed + `"}`, false},
		{"wrong-group-type", `{"id":"` + candidateID + `","scim_group_id":7,"workspace_id":"workspace","unknown":"` + undisclosed + `"}`, false},
		{"unstructured-id", `{"id":"` + undisclosed + `","scim_group_id":"group","workspace_id":"workspace"}`, false},
		{"null-id", `{"id":null,"scim_group_id":"group","workspace_id":"workspace","unknown":"` + undisclosed + `"}`, false},
		{"non-object-data", `"` + undisclosed + `"`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/scim/group-mappings" {
					t.Errorf("unexpected recovery action %s %s", r.Method, r.URL)
				}
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"data":`+test.data+`}`)
			}))
			defer server.Close()
			mapping, err := New("local-key", server.URL, "", time.Second).CreateSCIMGroupMapping(context.Background(), SCIMGroupMappingCreateRequest{"group", "workspace", "member"})
			if mapping != nil || err == nil || calls != 1 {
				t.Fatalf("mapping=%v err=%v calls=%d", mapping, err, calls)
			}
			message := err.Error()
			if strings.Contains(message, undisclosed) {
				t.Fatalf("response content leaked: %s", message)
			}
			if strings.Contains(message, candidateID) != test.wantCandidate {
				t.Fatalf("candidate diagnostic mismatch: %s", message)
			}
			if test.wantCandidate {
				for _, guidance := range []string{"Do not retry creation blindly", "Inspect and verify", "import it if appropriate", "no mapping was adopted"} {
					if !strings.Contains(message, guidance) {
						t.Fatalf("missing recovery guidance %q: %s", guidance, message)
					}
				}
			} else if !strings.Contains(message, "without a trustworthy matching candidate ID") {
				t.Fatalf("unsafe recovery guidance: %s", message)
			}
		})
	}
}

func TestSCIMMappingCreateHTTPFailureDoesNotSuggestRecoveryCandidate(t *testing.T) {
	const candidateID = "770e8400-e29b-41d4-a716-446655440000"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"error":{"message":"blocked"},"data":{"id":"`+candidateID+`","scim_group_id":"group","workspace_id":"workspace"}}`)
	}))
	defer server.Close()
	mapping, err := New("k", server.URL, "", time.Second).CreateSCIMGroupMapping(context.Background(), SCIMGroupMappingCreateRequest{"group", "workspace", "member"})
	if mapping != nil || err == nil || err.Error() != "openrouter API error (409): blocked" || calls != 1 {
		t.Fatalf("mapping=%v err=%v calls=%d", mapping, err, calls)
	}
}
