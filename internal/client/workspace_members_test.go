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

func TestWorkspaceMembersReadOnlyPagination(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/workspaces/ws/members" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		offset := r.URL.Query().Get("offset")
		id := "z"
		if offset == "1" {
			id = "a"
		} else if offset != "0" {
			t.Errorf("offset=%s", offset)
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []WorkspaceMember{{ID: "membership-" + id, WorkspaceID: "ws", UserID: id, Role: "restricted_viewer", CreatedAt: "now"}}, "total_count": 2})
	}))
	defer server.Close()
	items, err := New("k", server.URL, "", time.Second).ListWorkspaceMembers(context.Background(), "ws")
	if err != nil || len(items) != 2 || items[0].UserID != "a" || calls != 2 {
		t.Fatalf("items=%v err=%v calls=%d", items, err, calls)
	}
}
func TestWorkspaceMembersRejectsWrongOrMalformedIdentity(t *testing.T) {
	for _, member := range []WorkspaceMember{{ID: "membership", WorkspaceID: "other", UserID: "user", Role: "member", CreatedAt: "now"}, {ID: "", WorkspaceID: "ws", UserID: "user", Role: "member", CreatedAt: "now"}, {ID: "membership", WorkspaceID: "ws", UserID: "", Role: "member", CreatedAt: "now"}, {ID: "membership", WorkspaceID: "ws", UserID: "user", Role: "", CreatedAt: "now"}} {
		t.Run(fmt.Sprint(member), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]any{"data": []WorkspaceMember{member}, "total_count": 1})
			}))
			defer server.Close()
			if _, err := New("k", server.URL, "", time.Second).ListWorkspaceMembers(context.Background(), "ws"); err == nil {
				t.Fatal("accepted malformed membership")
			}
		})
	}
}
func TestWorkspaceMembersFailureIsNotEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `{"error":{"message":"SCIM ownership"}}`)
	}))
	defer server.Close()
	items, err := New("k", server.URL, "", time.Second).ListWorkspaceMembers(context.Background(), "ws")
	if items != nil || err == nil || !strings.HasPrefix(err.Error(), "openrouter API error (403)") {
		t.Fatalf("items=%v err=%v", items, err)
	}
}
