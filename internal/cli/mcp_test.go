package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mcpAnswer struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// mcpExchange sends the lines to a server and returns its answers, one per
// line written: a line that is not JSON fails the test.
func mcpExchange(t *testing.T, dir string, lines ...string) []mcpAnswer {
	t.Helper()
	var out bytes.Buffer
	server := mcpServer{version: "1.2.3", dir: dir, options: core.Options{Clones: true}, pullRequest: "none"}
	require.NoError(t, serveMCP(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, server))
	var answers []mcpAnswer
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var answer mcpAnswer
		require.NoError(t, json.Unmarshal([]byte(line), &answer), line)
		assert.Equal(t, "2.0", answer.JSONRPC)
		answers = append(answers, answer)
	}
	return answers
}

// mcpCall calls one tool on the repository and returns its result.
func mcpCall(t *testing.T, dir, name string, arguments map[string]any) mcpToolResult {
	t.Helper()
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}})
	require.NoError(t, err)
	answers := mcpExchange(t, dir, string(request))
	require.Len(t, answers, 1)
	require.Nil(t, answers[0].Error)
	var result mcpToolResult
	require.NoError(t, json.Unmarshal(answers[0].Result, &result))
	require.NotEmpty(t, result.Content)
	for _, content := range result.Content {
		assert.Equal(t, "text", content.Type)
	}
	return result
}

func TestMCPInitialize(t *testing.T) {
	answers := mcpExchange(t, ".",
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":"two","method":"initialize","params":{"protocolVersion":"1999-01-01"}}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	)
	require.Len(t, answers, 3, "the notification gets no answer")
	assert.Equal(t, "1", string(answers[0].ID))
	assert.JSONEq(t, `{"protocolVersion":"2025-03-26","capabilities":{"tools":{}},"serverInfo":{"name":"cribelo","version":"1.2.3"}}`, string(answers[0].Result))

	assert.Equal(t, `"two"`, string(answers[1].ID), "the id comes back as it was sent")
	var result struct{ ProtocolVersion string }
	require.NoError(t, json.Unmarshal(answers[1].Result, &result))
	assert.Equal(t, "2025-06-18", result.ProtocolVersion, "a revision this server does not know")

	assert.Equal(t, "{}", string(answers[2].Result))
}

func TestMCPToolsList(t *testing.T) {
	answers := mcpExchange(t, ".", `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	require.Len(t, answers, 1)
	var result struct {
		Tools []struct {
			Name        string
			Description string
			InputSchema struct {
				Type       string
				Properties map[string]struct{ Type, Description string }
				Required   []string
			}
			Annotations struct{ ReadOnlyHint bool }
		}
	}
	require.NoError(t, json.Unmarshal(answers[0].Result, &result))
	require.Len(t, result.Tools, 2)
	inputs := map[string][]string{
		"reading_plan": {"base", "dir", "fail_over"},
		"explain":      {"base", "dir", "path"},
	}
	for _, tool := range result.Tools {
		want, known := inputs[tool.Name]
		require.True(t, known, tool.Name)
		assert.True(t, tool.Annotations.ReadOnlyHint, tool.Name)
		assert.Contains(t, tool.Description, "Call ", tool.Name)
		assert.NotContains(t, tool.Description, string(rune(0x2014)), tool.Name)
		assert.Equal(t, "object", tool.InputSchema.Type, tool.Name)
		assert.Len(t, tool.InputSchema.Properties, len(want), tool.Name)
		for _, name := range want {
			property, declared := tool.InputSchema.Properties[name]
			require.True(t, declared, "%s.%s", tool.Name, name)
			assert.Equal(t, "string", property.Type)
			assert.NotEmpty(t, property.Description)
		}
		for _, name := range tool.InputSchema.Required {
			assert.Contains(t, tool.InputSchema.Properties, name, "a required input is a declared one")
		}
		if tool.Name == "explain" {
			assert.Equal(t, []string{"path"}, tool.InputSchema.Required)
		} else {
			assert.Empty(t, tool.InputSchema.Required, tool.Name)
		}
	}
}

func TestMCPToolsCallOnARepository(t *testing.T) {
	root, base := changedRepo(t)
	// A file the engine analyzes, next to the text files it skips.
	source := "package lib\n\nfunc Sign(n int) int {\n\tif n < 0 {\n\t\treturn -1\n\t}\n\treturn 1\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, "lib", "sign.go"), []byte(source), 0o644))

	result := mcpCall(t, root, "reading_plan", map[string]any{"base": base})
	assert.False(t, result.IsError, result.Content[0].Text)
	require.Len(t, result.Content, 1, "no gate asked for, none answered")
	want, err := buildReport(root, core.Options{Base: base, Clones: true}, "none")
	require.NoError(t, err)
	want.Version = "1.2.3"
	wantJSON, err := want.JSON()
	require.NoError(t, err)
	assert.JSONEq(t, wantJSON, result.Content[0].Text, "the JSON of cribelo --format json")
	assert.Contains(t, result.Content[0].Text, "a.txt")

	// The directory of the call wins over the one of the server.
	result = mcpCall(t, t.TempDir(), "reading_plan", map[string]any{"base": base, "dir": root, "fail_over": "1h"})
	assert.False(t, result.IsError, result.Content[0].Text)
	require.Len(t, result.Content, 2)
	assert.Equal(t, "gate: passed", result.Content[1].Text)

	result = mcpCall(t, root, "explain", map[string]any{"base": base, "path": "lib/sign.go"})
	assert.False(t, result.IsError, result.Content[0].Text)
	wantText, err := want.Explain("lib/sign.go")
	require.NoError(t, err)
	assert.Equal(t, wantText, result.Content[0].Text, "the text of cribelo explain")
	assert.Contains(t, result.Content[0].Text, "Signals")

	// A path given from the directory of the call, not from the root.
	result = mcpCall(t, root, "explain", map[string]any{"base": base, "dir": filepath.Join(root, "lib"), "path": "sign.go"})
	assert.False(t, result.IsError, result.Content[0].Text)
	assert.Equal(t, wantText, result.Content[0].Text)
}

func TestMCPToolFailureIsAResult(t *testing.T) {
	root, base := changedRepo(t)
	for name, arguments := range map[string]map[string]any{
		"reading_plan": {"base": base, "fail_over": "soon"},
		"explain":      {"base": base},
	} {
		result := mcpCall(t, root, name, arguments)
		assert.True(t, result.IsError, name)
	}
	result := mcpCall(t, root, "explain", map[string]any{"base": base, "path": "nowhere.txt"})
	assert.True(t, result.IsError)
	result = mcpCall(t, t.TempDir(), "reading_plan", nil)
	assert.True(t, result.IsError, "not a repository")

	answers := mcpExchange(t, root, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rewrite"}}`)
	require.Len(t, answers, 1)
	require.NotNil(t, answers[0].Error, "a tool this server does not have")
	assert.Equal(t, rpcInvalidParams, answers[0].Error.Code)
}

func TestMCPErrors(t *testing.T) {
	answers := mcpExchange(t, ".",
		`{"jsonrpc":"2.0","id":7,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":7}}`,
		`{"jsonrpc":"2.0","method":"resources/list"}`,
		``,
		`{"jsonrpc":"2.0","id":8,"method":`,
		`[{"jsonrpc":"2.0","id":9,"method":"ping"}]`,
		`{"jsonrpc":"2.0","id":10,"result":{}}`,
		`{"jsonrpc":"2.0","id":11,"method":"ping"}`,
	)
	require.Len(t, answers, 4, "neither the notifications nor the answer of the client are answered")

	require.NotNil(t, answers[0].Error)
	assert.Equal(t, rpcMethodNotFound, answers[0].Error.Code)
	assert.Equal(t, "7", string(answers[0].ID))
	assert.Empty(t, answers[0].Result)

	require.NotNil(t, answers[1].Error)
	assert.Equal(t, rpcParseError, answers[1].Error.Code)
	assert.Equal(t, "null", string(answers[1].ID))

	require.NotNil(t, answers[2].Error)
	assert.Equal(t, rpcInvalidRequest, answers[2].Error.Code)

	assert.Nil(t, answers[3].Error, "the server is still up after the errors")
	assert.Equal(t, "11", string(answers[3].ID))
}

// changedRepo is a repository with a base commit and a working tree that
// changes a file, adds two and removes one.
func changedRepo(t *testing.T) (root, base string) {
	t.Helper()
	root = t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	write := func(rel, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644))
	}
	run("init", "-q", "-b", "main")
	run("config", "user.name", "t")
	run("config", "user.email", "t@example.com")
	write("a.txt", "one\n")
	write("old.txt", "gone soon\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	base = run("rev-parse", "HEAD")
	run("checkout", "-q", "-b", "feature")
	write("a.txt", "two\n")
	write("lib/b.txt", "b\n")
	write("c.txt", "c\n")
	require.NoError(t, os.Remove(filepath.Join(root, "old.txt")))
	return root, base
}
