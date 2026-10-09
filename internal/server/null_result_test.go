package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// createNullFunction creates api.null_result (returns SQL NULL) and grants it
// to the test user.
func createNullFunction(env *testEnv) error {
	db, err := sql.Open("postgres", buildConnStr(env.cfg, env.dbName))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec(fmt.Sprintf(`
CREATE OR REPLACE FUNCTION api.null_result(payload jsonb) RETURNS json
LANGUAGE sql AS $$ SELECT NULL::json $$;
GRANT EXECUTE ON FUNCTION api.null_result(jsonb) TO %s;`, env.testUser))
	return err
}

// A function returning SQL NULL (for example a lookup that found no row) must
// yield "result": null, not a 500.
func TestFunctionReturningNullIsJSONNull(t *testing.T) {
	env := requireTestEnv(t)
	defer env.close()
	token := loginAndGetTokenOrSkip(t, env)
	if err := createNullFunction(env); err != nil {
		t.Fatalf("create function: %v", err)
	}

	t.Run("jsonrpc", func(t *testing.T) {
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "api.null_result", "params": map[string]any{}, "id": 1})
		req, _ := http.NewRequest(http.MethodPost, env.apiURL(), bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d: %s", resp.StatusCode, body)
		}
		var out map[string]json.RawMessage
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		if string(out["result"]) != "null" {
			t.Errorf("result = %s; want null (body %s)", out["result"], body)
		}
	})

	t.Run("mcp", func(t *testing.T) {
		resp := mcpDo(t, env.mcpURL(), token, map[string]interface{}{
			"jsonrpc": "2.0", "id": 1, "method": "tools/call",
			"params": map[string]interface{}{"name": "api.null_result", "arguments": map[string]interface{}{}},
		})
		out := decodeMCPResponse(t, resp)
		result, _ := out["result"].(map[string]interface{})
		if isErr, _ := result["isError"].(bool); isErr {
			t.Fatalf("tool error for NULL result: %v", out)
		}
		content, _ := result["content"].([]interface{})
		if len(content) != 1 || !strings.Contains(fmt.Sprint(content[0]), "null") {
			t.Errorf("content = %v; want a text block with null", content)
		}
	})
}
