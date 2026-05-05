package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

func TestCheckGraphQL_BasicIntrospection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json content-type, got %s", ct)
		}

		resp := map[string]any{
			"data": map[string]any{
				"__schema": map[string]any{
					"types": []map[string]any{
						{"name": "Query"},
						{"name": "Mutation"},
						{"name": "User"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	check := &schema.GraphQLCheck{URL: server.URL}
	results := CheckGraphQL(check)

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if !results[0].Passed {
		t.Errorf("expected introspection to pass, got: %s", results[0].Actual)
	}
}

func TestCheckGraphQL_ConnectionFailure(t *testing.T) {
	check := &schema.GraphQLCheck{URL: "http://127.0.0.1:1/graphql"}
	results := CheckGraphQL(check)

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Passed {
		t.Error("expected connection failure to fail")
	}
}

func TestCheckGraphQL_StatusCodeMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{"errors": []map[string]any{{"message": "boom"}}})
	}))
	defer server.Close()

	statusCode := 200
	check := &schema.GraphQLCheck{URL: server.URL, StatusCode: &statusCode}
	results := CheckGraphQL(check)

	// Find status code result
	found := false
	for _, r := range results {
		if r.Type == "graphql_status" && !r.Passed {
			found = true
		}
	}
	if !found {
		t.Error("expected status code mismatch to fail")
	}
}

func TestCheckGraphQL_ExpectTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"__schema": map[string]any{
					"types": []map[string]any{
						{"name": "Query"},
						{"name": "Mutation"},
						{"name": "User"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	check := &schema.GraphQLCheck{
		URL:         server.URL,
		ExpectTypes: []string{"Query", "Mutation", "User"},
	}
	results := CheckGraphQL(check)

	// All results should pass
	for _, r := range results {
		if !r.Passed {
			t.Errorf("expected all checks to pass, got failure: %s - %s", r.Type, r.Actual)
		}
	}
}

func TestCheckGraphQL_ExpectTypesMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"__schema": map[string]any{
					"types": []map[string]any{
						{"name": "Query"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	check := &schema.GraphQLCheck{
		URL:         server.URL,
		ExpectTypes: []string{"Query", "Subscription"},
	}
	results := CheckGraphQL(check)

	// Should have a failure for missing "Subscription"
	allPassed := true
	for _, r := range results {
		if r.Type == "graphql_types" && !r.Passed {
			allPassed = false
		}
	}
	if allPassed {
		t.Error("expected missing type 'Subscription' to fail")
	}
}

func TestCheckGraphQL_GraphQLErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "introspection not allowed"}},
		})
	}))
	defer server.Close()

	check := &schema.GraphQLCheck{URL: server.URL}
	results := CheckGraphQL(check)

	// Find graphql_errors result
	found := false
	for _, r := range results {
		if r.Type == "graphql_errors" && !r.Passed {
			found = true
		}
	}
	if !found {
		t.Error("expected GraphQL errors check to fail")
	}
}

func TestCheckGraphQL_ExpectContains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"__schema": map[string]any{
					"types": []map[string]any{
						{"name": "Query"},
						{"name": "User"},
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	check := &schema.GraphQLCheck{
		URL:            server.URL,
		ExpectContains: "User",
	}
	results := CheckGraphQL(check)

	// Find body_contains result
	found := false
	for _, r := range results {
		if r.Type == "graphql_contains" {
			found = true
			if !r.Passed {
				t.Errorf("expected body to contain 'User', got: %s", r.Actual)
			}
		}
	}
	if !found {
		t.Error("expected graphql_contains result")
	}
}

func TestCheckGraphQL_CustomQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		query := body["query"].(string)

		if query != "{ __typename }" {
			t.Errorf("expected custom query, got: %s", query)
		}

		resp := map[string]any{
			"data": map[string]any{"__typename": "Query"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	check := &schema.GraphQLCheck{
		URL:   server.URL,
		Query: "{ __typename }",
	}
	results := CheckGraphQL(check)

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if !results[0].Passed {
		t.Errorf("expected custom query to pass, got: %s", results[0].Actual)
	}
}
