package cursorhooks

// Claude Code speaks almost the same protocol as Cursor (a command per agent
// step, JSON on stdin, a decision on stdout), with different names and a
// different answer shape (https://code.claude.com/docs/en/hooks). This file
// turns its events into the calls Handler already knows, so the policy, the
// approvals, the budget, the log and the cockpit need no special case.
//
// What differs from Cursor, and why the code is not a rename:
//   - One PreToolUse covers every tool, edits included, so a file write is
//     gated BEFORE it happens (Cursor only reports it afterwards).
//   - An explicit "allow" makes Claude Code skip its own permission prompt.
//     Sentinel therefore says nothing about a call it has no objection to, so
//     Claude's normal rules still apply, and says "allow" only when a human
//     approved the call in the cockpit.
//   - Stop does not fire when the user interrupts, and a cancelled tool sends
//     no PostToolUse. Calls left open are closed at the next Stop or at the end
//     of the session.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/zaninifrancesco/agent-sentinel/internal/policy"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

// Claude Code hook events Handler acts on.
const (
	EventClaudePre         = "PreToolUse"
	EventClaudePost        = "PostToolUse"
	EventClaudePostFailure = "PostToolUseFailure"
	EventClaudeStop        = "Stop"
	EventClaudeSessionEnd  = "SessionEnd"
)

// ClaudeEvents are the events `sentinel hook install --agent claude` registers.
var ClaudeEvents = []string{EventClaudePre, EventClaudePost, EventClaudePostFailure, EventClaudeStop, EventClaudeSessionEnd}

// ClaudeToolMatcher selects the tools Sentinel supervises: the shell, files and
// the web, and with mcp every MCP tool too (left out by default: a server that
// already goes through `sentinel mcp` would be judged twice). Bookkeeping tools
// (todo lists, plan mode, asking the user a question) are never selected, there
// is nothing in them to judge.
func ClaudeToolMatcher(mcp bool) string {
	tools := "Bash|PowerShell|Read|Edit|Write|MultiEdit|NotebookEdit|Glob|Grep|WebFetch|WebSearch"
	if mcp {
		tools += "|mcp__.*"
	}
	return "^(" + tools + ")$"
}

func isClaudeEvent(name string) bool {
	switch name {
	case EventClaudePre, EventClaudePost, EventClaudePostFailure, EventClaudeStop, EventClaudeSessionEnd:
		return true
	}
	return false
}

// claudeOutput is what Claude Code reads back from the hook.
type claudeOutput struct {
	HookSpecificOutput *claudeDecision `json:"hookSpecificOutput,omitempty"`
	SystemMessage      string          `json:"systemMessage,omitempty"` // shown to the user
}

type claudeDecision struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"` // allow | deny | ask
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"` // added to the model's context
}

func (h *Handler) serveClaude(w http.ResponseWriter, r *http.Request, in input) {
	in.ConversationID = in.SessionID
	var out claudeOutput
	switch in.Event {
	case EventClaudePre:
		out = h.claudeBefore(r.Context(), in)
	case EventClaudePost:
		h.claudeAfter(in, false)
	case EventClaudePostFailure:
		h.claudeAfter(in, true)
	case EventClaudeStop, EventClaudeSessionEnd:
		h.claudeClose(in)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}

func (h *Handler) claudeBefore(ctx context.Context, in input) claudeOutput {
	tool, args, call := claudeCall(in)
	return toClaude(h.gate(ctx, in, tool, args, claudeKey(in), call))
}

// toClaude translates the decision made for a call into Claude Code's schema.
func toClaude(resp Response) claudeOutput {
	switch {
	case resp.Permission == "deny":
		return claudeOutput{
			SystemMessage: resp.UserMessage,
			HookSpecificOutput: &claudeDecision{
				HookEventName: EventClaudePre, PermissionDecision: "deny", PermissionDecisionReason: resp.AgentMessage,
			},
		}
	case resp.approved:
		return claudeOutput{HookSpecificOutput: &claudeDecision{
			HookEventName: EventClaudePre, PermissionDecision: "allow",
			PermissionDecisionReason: "approved in the Agent Sentinel cockpit",
			AdditionalContext:        resp.AgentMessage,
		}}
	}
	return claudeOutput{} // no objection: Claude Code's own permission rules decide
}

// claudeKey pairs a PreToolUse with its PostToolUse. Claude Code names the tool
// use, so two identical commands never get mixed up.
func claudeKey(in input) string {
	if in.ToolUseID != "" {
		return in.ToolUseID
	}
	return in.ToolName + "\x00" + string(in.ToolInput)
}

// claudeToolInput is the union of the tool_input fields of the tools we name.
type claudeToolInput struct {
	Command      string     `json:"command"`
	FilePath     string     `json:"file_path"`
	NotebookPath string     `json:"notebook_path"`
	Content      string     `json:"content"`    // Write
	OldString    string     `json:"old_string"` // Edit
	NewString    string     `json:"new_string"`
	NewSource    string     `json:"new_source"` // NotebookEdit
	Edits        []fileEdit `json:"edits"`      // MultiEdit
}

// claudeCall names a tool call the way the policy and the cockpit know it, with
// the arguments to record and the call the policy should judge.
func claudeCall(in input) (tool string, args map[string]any, call policy.Call) {
	var ti claudeToolInput
	_ = json.Unmarshal(in.ToolInput, &ti)

	switch in.ToolName {
	case "Bash", "PowerShell":
		in.Command = ti.Command
		tool, args = "shell", shellArgs(in)
	case "Read":
		tool, args = "read_file", map[string]any{"file_path": ti.FilePath}
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		path := ti.FilePath
		var edits []fileEdit
		switch in.ToolName {
		case "Edit":
			edits = []fileEdit{{Old: ti.OldString, New: ti.NewString}}
		case "Write":
			edits = []fileEdit{{New: ti.Content}}
		case "MultiEdit":
			edits = ti.Edits
		case "NotebookEdit":
			path, edits = ti.NotebookPath, []fileEdit{{New: ti.NewSource}}
		}
		tool = "edit_file"
		// "file_path" sorts before "patch", so the cockpit shows the file as the row subtitle.
		args = map[string]any{"file_path": path, "patch": truncate(renderDiff(path, edits))}
		// The policy reads the path and what is being written, never the text
		// that is being replaced: deleting a secret is not leaking one.
		call = policy.Call{Tool: tool, Texts: []string{path}}
		for _, e := range edits {
			call.Texts = append(call.Texts, e.New)
		}
		return tool, args, call
	default:
		if rest, ok := strings.CutPrefix(in.ToolName, "mcp__"); ok {
			if server, name, ok := strings.Cut(rest, "__"); ok {
				tool = server + ":" + name
			} else {
				tool = rest
			}
		} else {
			tool = in.ToolName
		}
		args = parseArgs(in.ToolInput)
	}
	return tool, args, policy.NewCall(tool, args)
}

// claudeAfter records how a tool call ended: PostToolUse after it worked,
// PostToolUseFailure after it failed.
func (h *Handler) claudeAfter(in input, failed bool) {
	tool, args, _ := claudeCall(in)
	status := recorder.StatusOK
	var text string
	size := len(in.ToolResponse)

	switch {
	case failed:
		status, text, size = recorder.StatusError, in.Error, len(in.Error)
		if in.IsInterrupt {
			status, text = recorder.StatusInterrupted, "Interrupted: "+text
		}
	case tool == "shell":
		var interrupted bool
		text, interrupted = bashResult(in.ToolResponse)
		if interrupted {
			status = recorder.StatusInterrupted
		}
	case tool == "read_file":
		// The file went to the model; its content must not go to the log.
		text = fmt.Sprintf("read %v (%d bytes returned to the model, not stored)", args["file_path"], size)
	case tool == "edit_file":
		text = fmt.Sprintf("edit applied to %v", args["file_path"])
	default:
		text = rawText(in.ToolResponse)
	}
	h.completeSized(in, claudeKey(in), tool, args, text, size, status, int64(in.DurationMS))
}

// bashResult reads Bash's tool_response: stdout and stderr, and whether the
// command was cut short.
func bashResult(raw json.RawMessage) (text string, interrupted bool) {
	var res struct {
		Stdout      string `json:"stdout"`
		Stderr      string `json:"stderr"`
		Interrupted bool   `json:"interrupted"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return rawText(raw), false
	}
	text = res.Stdout
	if res.Stderr != "" {
		text += "\n[stderr]\n" + res.Stderr
	}
	if res.Interrupted {
		text += "\n[the command was interrupted]"
	}
	return text, res.Interrupted
}

// rawText is a tool result as text: a JSON string unquoted, anything else as is.
func rawText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

// claudeClose closes what a finished turn or session left open.
func (h *Handler) claudeClose(in input) {
	why := "Claude Code ended the turn"
	if in.Event == EventClaudeSessionEnd {
		why = "The Claude Code session ended"
		if in.EndReason != "" {
			why += " (" + in.EndReason + ")"
		}
	}
	n := h.closeOpen(in.SessionID, why)
	if h.Logf != nil {
		h.Logf("Claude Code sent %s for session %s; closed %d unfinished call(s)", in.Event, in.SessionID, n)
	}
}
