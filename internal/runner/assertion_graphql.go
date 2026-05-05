package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

const defaultIntrospectionQuery = `{"query":"{ __schema { types { name } } }"}`

// CheckGraphQL sends an introspection query to a GraphQL endpoint and validates the response.
func CheckGraphQL(check *schema.GraphQLCheck) []AssertionResult {
	var results []AssertionResult

	timeout := 10 * time.Second
	if check.Timeout.Duration > 0 {
		timeout = check.Timeout.Duration
	}
	client := &http.Client{Timeout: timeout}

	body := defaultIntrospectionQuery
	if check.Query != "" {
		b, _ := json.Marshal(map[string]string{"query": check.Query})
		body = string(b)
	}

	req, err := http.NewRequest("POST", check.URL, bytes.NewReader([]byte(body)))
	if err != nil {
		return []AssertionResult{{
			Type:     "graphql_request",
			Expected: check.URL,
			Actual:   fmt.Sprintf("invalid request: %v", err),
			Passed:   false,
		}}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return []AssertionResult{{
			Type:     "graphql_request",
			Expected: check.URL,
			Actual:   fmt.Sprintf("request failed: %v", err),
			Passed:   false,
		}}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return []AssertionResult{{
			Type:     "graphql_request",
			Expected: "readable body",
			Actual:   fmt.Sprintf("failed to read body: %v", err),
			Passed:   false,
		}}
	}

	// Check status code
	expectedStatus := 200
	if check.StatusCode != nil {
		expectedStatus = *check.StatusCode
	}
	results = append(results, AssertionResult{
		Type:     "graphql_status",
		Expected: fmt.Sprintf("HTTP %d", expectedStatus),
		Actual:   fmt.Sprintf("HTTP %d", resp.StatusCode),
		Passed:   resp.StatusCode == expectedStatus,
	})

	// Parse response
	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &gqlResp); err != nil {
		results = append(results, AssertionResult{
			Type:     "graphql_parse",
			Expected: "valid JSON response",
			Actual:   fmt.Sprintf("parse error: %v", err),
			Passed:   false,
		})
		return results
	}

	// Check for GraphQL errors
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, len(gqlResp.Errors))
		for i, e := range gqlResp.Errors {
			msgs[i] = e.Message
		}
		results = append(results, AssertionResult{
			Type:     "graphql_errors",
			Expected: "no errors",
			Actual:   strings.Join(msgs, "; "),
			Passed:   false,
		})
		return results
	}

	// Check for expected types (only works with standard introspection response)
	if len(check.ExpectTypes) > 0 {
		var schemaData struct {
			Schema struct {
				Types []struct {
					Name string `json:"name"`
				} `json:"types"`
			} `json:"__schema"`
		}
		if err := json.Unmarshal(gqlResp.Data, &schemaData); err == nil {
			typeSet := make(map[string]bool)
			for _, t := range schemaData.Schema.Types {
				typeSet[t.Name] = true
			}
			var missing []string
			for _, expected := range check.ExpectTypes {
				if !typeSet[expected] {
					missing = append(missing, expected)
				}
			}
			if len(missing) > 0 {
				results = append(results, AssertionResult{
					Type:     "graphql_types",
					Expected: fmt.Sprintf("types: %s", strings.Join(check.ExpectTypes, ", ")),
					Actual:   fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
					Passed:   false,
				})
			} else {
				results = append(results, AssertionResult{
					Type:     "graphql_types",
					Expected: fmt.Sprintf("types: %s", strings.Join(check.ExpectTypes, ", ")),
					Actual:   "all found",
					Passed:   true,
				})
			}
		}
	}

	// Check body contains
	if check.ExpectContains != "" {
		results = append(results, AssertionResult{
			Type:     "graphql_contains",
			Expected: fmt.Sprintf("contains %q", check.ExpectContains),
			Actual:   string(respBody),
			Passed:   strings.Contains(string(respBody), check.ExpectContains),
		})
	}

	// If no specific checks beyond status, add a basic success result
	if len(check.ExpectTypes) == 0 && check.ExpectContains == "" {
		results = append(results, AssertionResult{
			Type:     "graphql_introspection",
			Expected: "introspection succeeds",
			Actual:   "data returned",
			Passed:   true,
		})
	}

	return results
}
