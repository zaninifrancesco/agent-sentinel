#!/usr/bin/env python3
"""Tiny fake MCP server (stdio, newline-delimited JSON-RPC) for trying out Sentinel."""
import json
import sys
import time

print("fake-mcp-server: booting (this line is not JSON-RPC)", flush=True)


def reply(msg_id, result=None, error=None):
    msg = {"jsonrpc": "2.0", "id": msg_id}
    if error:
        msg["error"] = error
    else:
        msg["result"] = result
    print(json.dumps(msg), flush=True)


def text(s, is_error=False):
    r = {"content": [{"type": "text", "text": s}]}
    if is_error:
        r["isError"] = True
    return r


for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    msg = json.loads(line)
    method, msg_id = msg.get("method"), msg.get("id")
    if msg_id is None:
        continue  # notification
    if method == "initialize":
        reply(msg_id, {"protocolVersion": "2025-06-18", "serverInfo": {"name": "fake", "version": "0.0.1"},
                       "capabilities": {"tools": {}}})
    elif method == "tools/list":
        reply(msg_id, {"tools": [{"name": n, "description": d, "inputSchema": {"type": "object"}} for n, d in
                                 [("read_file", "Read a file"), ("run_shell", "Run a command"),
                                  ("slow_search", "Takes a while"), ("boom", "Always fails")]]})
    elif method == "tools/call":
        name = msg["params"]["name"]
        args = msg["params"].get("arguments", {})
        if name == "read_file":
            reply(msg_id, text(f"contents of {args.get('path')}"))
        elif name == "run_shell":
            time.sleep(0.05)
            reply(msg_id, text(f"$ {args.get('command')}\nok"))
        elif name == "slow_search":
            time.sleep(0.6)
            reply(msg_id, text("42 results"))
        elif name == "boom":
            reply(msg_id, text("something went wrong", is_error=True))
        else:
            reply(msg_id, error={"code": -32602, "message": f"unknown tool {name}"})
    else:
        reply(msg_id, error={"code": -32601, "message": f"method not found: {method}"})
