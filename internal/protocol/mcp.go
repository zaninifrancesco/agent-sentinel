package protocol

import "encoding/json"

// MCP method names that Sentinel cares about.
const (
	MethodInitialize    = "initialize"
	MethodToolsList     = "tools/list"
	MethodToolsCall     = "tools/call"
	MethodResourcesRead = "resources/read"
	MethodPromptsGet    = "prompts/get"
)

// CallToolParams is the `params` of a tools/call request.
type CallToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Content is one block of a tool result (text, image, resource...).
type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

// CallToolResult is the `result` of a tools/call response.
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Tool describes a tool exposed by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ListToolsResult is the `result` of a tools/list response.
type ListToolsResult struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ReadResourceParams is the `params` of a resources/read request.
type ReadResourceParams struct {
	URI string `json:"uri"`
}

// AsCallTool decodes the params of a tools/call request.
func (m *Message) AsCallTool() (*CallToolParams, error) {
	var p CallToolParams
	if err := json.Unmarshal(m.Params, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// AsCallToolResult decodes the result of a tools/call response.
func (m *Message) AsCallToolResult() (*CallToolResult, error) {
	var r CallToolResult
	if err := json.Unmarshal(m.Result, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// AsListToolsResult decodes the result of a tools/list response.
func (m *Message) AsListToolsResult() (*ListToolsResult, error) {
	var r ListToolsResult
	if err := json.Unmarshal(m.Result, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
