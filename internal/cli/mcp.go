package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	core "github.com/ast-metrics/ast-metrics/cribelo"
	"github.com/cribelo/cribelo/internal/render"
)

// The Model Context Protocol is JSON-RPC 2.0, one message per line on
// stdio. The three methods a tool server needs are small enough to be
// written here, which keeps the binary free of an SDK.
const (
	rpcParseError     = -32700
	rpcInvalidRequest = -32600
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
	rpcInternalError  = -32603
)

// mcpProtocols are the revisions of the protocol this server can speak; a
// client that asks for another one is answered with the default.
var mcpProtocols = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

const mcpDefaultProtocol = "2025-06-18"

// mcpServer carries what the command line was started with: every tool
// call starts from these options, and its arguments are laid over them.
type mcpServer struct {
	version     string
	dir         string
	options     core.Options
	pullRequest string
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	// Set on the answers of the client, which are not requests.
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// rpcResponse holds its result already encoded: an empty object is a
// result (the one of ping), and omitempty would drop it as a map.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// mcpTool is one tool: what tools/list says of it, and what runs it. The
// texts it returns are the content blocks of the result.
type mcpTool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations map[string]any `json:"annotations"`

	call func(s mcpServer, args mcpArguments) ([]string, error)
}

// mcpArguments are the inputs of the three tools together; each tool reads
// the ones its schema declares.
type mcpArguments struct {
	Base     string `json:"base"`
	Dir      string `json:"dir"`
	FailOver string `json:"fail_over"`
	Budget   string `json:"budget"`
	Path     string `json:"path"`
}

// The descriptions are written for the agent that picks the tool: when to
// call it comes first, what comes back second.
func mcpTools() []mcpTool {
	base := map[string]any{"type": "string", "description": "Git reference to compare the working tree against, e.g. main or a commit. Default: the default branch of the repository."}
	dir := map[string]any{"type": "string", "description": "Repository, or a directory inside it. Default: the directory the server was started in."}
	schema := func(required []string, properties map[string]any) map[string]any {
		properties["base"], properties["dir"] = base, dir
		s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			s["required"] = required
		}
		return s
	}
	readOnly := map[string]any{"readOnlyHint": true}
	return []mcpTool{
		{
			Name:  "reading_plan",
			Title: "Reading plan of the change",
			Description: "Call before opening a pull request, once the change is written: reading time of the whole change, " +
				"the files and line ranges a reviewer must read closely, the ones to glance at, the files to skip, " +
				"and the facts the diff does not show. Covers the working tree against the base, uncommitted changes included. " +
				"Give fail_over to learn whether the change fits the reading time your team accepts. Returns JSON.",
			InputSchema: schema(nil, map[string]any{
				"fail_over": map[string]any{"type": "string", "description": "Longest reading time accepted for the whole change, as a duration such as 30m or 1h. When given, the answer says whether the gate passed."},
			}),
			Annotations: readOnly,
			call:        mcpReadingPlan,
		},
		{
			Name:  "explain",
			Title: "Signals behind one file",
			Description: "Call when reading_plan puts a file where you did not expect it, to read closely or to skip, " +
				"and you want to know why before you rework it: every signal behind the position of the file, " +
				"with its weight, and its changed line ranges. Returns text.",
			InputSchema: schema([]string{"path"}, map[string]any{
				"path": map[string]any{"type": "string", "description": "A file of the change, relative to the root of the repository."},
			}),
			Annotations: readOnly,
			call:        mcpExplain,
		},
	}
}

// serveMCP answers the messages read from in, one per line, until in is
// closed. Only protocol messages are written to out: a client reads every
// line of it as JSON.
func serveMCP(in io.Reader, out io.Writer, s mcpServer) error {
	reader := bufio.NewReader(in)
	encoder := json.NewEncoder(out)
	for {
		// A plan of a large change is longer than the buffer of a
		// bufio.Scanner, and so may a request be.
		line, readErr := reader.ReadBytes('\n')
		if line = bytes.TrimSpace(line); len(line) > 0 {
			if response := s.handle(line); response != nil {
				if err := encoder.Encode(response); err != nil {
					return err
				}
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// handle answers one message; nil when the message is a notification,
// which never gets an answer, whatever it asks.
func (s mcpServer) handle(line []byte) *rpcResponse {
	null := json.RawMessage("null")
	var request rpcRequest
	if err := json.Unmarshal(line, &request); err != nil {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) {
			return &rpcResponse{JSONRPC: "2.0", ID: null, Error: &rpcError{rpcParseError, "parse error: " + err.Error()}}
		}
		// Valid JSON of another shape, a batch for one.
		return &rpcResponse{JSONRPC: "2.0", ID: null, Error: &rpcError{rpcInvalidRequest, "invalid request: one JSON-RPC message per line"}}
	}
	if request.Method == "" {
		// An answer of the client to a request this server never sent.
		if len(request.ID) == 0 || len(request.Result) > 0 || len(request.Error) > 0 {
			return nil
		}
		return &rpcResponse{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{rpcInvalidRequest, "invalid request: no method"}}
	}
	result, rpcErr := s.dispatch(request)
	if len(request.ID) == 0 {
		return nil
	}
	if rpcErr != nil {
		return &rpcResponse{JSONRPC: "2.0", ID: request.ID, Error: rpcErr}
	}
	if result == nil {
		result = map[string]any{}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return &rpcResponse{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{rpcInternalError, err.Error()}}
	}
	return &rpcResponse{JSONRPC: "2.0", ID: request.ID, Result: data}
}

func (s mcpServer) dispatch(request rpcRequest) (any, *rpcError) {
	switch request.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if len(request.Params) > 0 {
			if err := json.Unmarshal(request.Params, &params); err != nil {
				return nil, &rpcError{rpcInvalidParams, "invalid params: " + err.Error()}
			}
		}
		protocol := mcpDefaultProtocol
		for _, known := range mcpProtocols {
			if params.ProtocolVersion == known {
				protocol = known
			}
		}
		return map[string]any{
			"protocolVersion": protocol,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "cribelo", "version": s.version},
		}, nil
	case "notifications/initialized":
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": mcpTools()}, nil
	case "tools/call":
		return s.callTool(request.Params)
	}
	return nil, &rpcError{rpcMethodNotFound, "method not found: " + request.Method}
}

// callTool runs one tool. A tool that fails is an answer like another, with
// isError set, so that the agent reads the message and corrects its call;
// only a call that names no tool of this server is a protocol error.
func (s mcpServer) callTool(raw json.RawMessage) (any, *rpcError) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, &rpcError{rpcInvalidParams, "invalid params: " + err.Error()}
	}
	for _, tool := range mcpTools() {
		if tool.Name != params.Name {
			continue
		}
		var args mcpArguments
		if len(params.Arguments) > 0 && string(params.Arguments) != "null" {
			if err := json.Unmarshal(params.Arguments, &args); err != nil {
				return toolFailure(fmt.Errorf("invalid arguments: %w", err)), nil
			}
		}
		texts, err := runTool(tool, s, args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cribelo mcp: %s: %v\n", tool.Name, err)
			return toolFailure(err), nil
		}
		result := mcpToolResult{}
		for _, text := range texts {
			result.Content = append(result.Content, mcpContent{Type: "text", Text: text})
		}
		return result, nil
	}
	return nil, &rpcError{rpcInvalidParams, "unknown tool: " + params.Name}
}

// runTool keeps the server up when the analysis panics on a repository: the
// agent gets the failure, and can still call the other tools.
func runTool(tool mcpTool, s mcpServer, args mcpArguments) (texts []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s failed: %v", tool.Name, r)
		}
	}()
	return tool.call(s, args)
}

func toolFailure(err error) mcpToolResult {
	return mcpToolResult{Content: []mcpContent{{Type: "text", Text: err.Error()}}, IsError: true}
}

// report builds the plan a tool answers from: the options of the server,
// then the base and the directory of the call.
func (s mcpServer) report(args mcpArguments, lay func(*core.Options) error) (render.Report, string, error) {
	opts, dir := s.options, s.dir
	if args.Base != "" {
		opts.Base = args.Base
	}
	if args.Dir != "" {
		dir = args.Dir
	}
	if lay != nil {
		if err := lay(&opts); err != nil {
			return render.Report{}, dir, err
		}
	}
	report, err := buildReport(dir, opts, s.pullRequest)
	report.Version = s.version
	return report, dir, err
}

func mcpDuration(name, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s: %q is not a duration (write 30m or 1h)", name, value)
	}
	return d, nil
}

func mcpReadingPlan(s mcpServer, args mcpArguments) ([]string, error) {
	report, _, err := s.report(args, func(opts *core.Options) error {
		if args.FailOver == "" {
			return nil
		}
		d, err := mcpDuration("fail_over", args.FailOver)
		opts.FailOver = d
		return err
	})
	if err != nil {
		return nil, err
	}
	out, err := report.JSON()
	if err != nil {
		return nil, err
	}
	texts := []string{out}
	// The plan carries the gate too; said apart, an agent does not have to
	// look for it in a long answer.
	if args.FailOver != "" && report.Gate != "" {
		texts = append(texts, "gate: "+report.Gate)
	}
	return texts, nil
}

func mcpExplain(s mcpServer, args mcpArguments) ([]string, error) {
	if args.Path == "" {
		return nil, errors.New("path is required: a file of the change, relative to the root of the repository")
	}
	report, dir, err := s.report(args, nil)
	if err != nil {
		return nil, err
	}
	// The path is read from the root of the repository first, the way the
	// plan writes it, then from dir, the way the command line reads it.
	rel := filepath.ToSlash(filepath.Clean(args.Path))
	out, err := report.Explain(rel)
	if err != nil {
		repo, repoErr := core.OpenRepo(dir)
		if repoErr != nil {
			return nil, err
		}
		path := args.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		other := relToRoot(repo.Root, path)
		if other == rel {
			return nil, err
		}
		if out, err = report.Explain(other); err != nil {
			return nil, err
		}
	}
	return []string{out}, nil
}
