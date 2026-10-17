import json
from datetime import datetime, timezone, timedelta

base_time = datetime(2026, 10, 16, 14, 22, 10, 120000, tzinfo=timezone.utc)
rec_id = "trace-8848a"
agent_session = "jwt-session-rotation"

calls_data = [
    {
        "tool": "read_file",
        "dur": 18,
        "offset": 0,
        "status": "ok",
        "args": {"path": "src/auth/jwt.ts"},
        "res": {
            "content": [
                {
                    "type": "text",
                    "text": "export interface TokenClaims {\n  sub: string;\n  email: string;\n  role: 'admin' | 'user';\n  iat: number;\n  exp: number;\n}\n\nexport function verifyToken(token: string): TokenClaims {\n  return jwt.verify(token, process.env.JWT_SECRET) as TokenClaims;\n}"
                }
            ]
        }
    },
    {
        "tool": "read_file",
        "dur": 14,
        "offset": 8, # slight overlap with #1 for concurrency = 2
        "status": "ok",
        "args": {"path": "src/config/auth.json"},
        "res": {"content": [{"type": "text", "text": "{\n  \"accessTokenTtl\": \"15m\",\n  \"refreshTokenTtl\": \"7d\",\n  \"algorithm\": \"HS256\"\n}"}]}
    },
    {
        "tool": "Grep",
        "dur": 42,
        "offset": 90,
        "status": "ok",
        "args": {"query": "verifyToken src/", "path": "src/server/routes.ts"},
        "res": {"content": [{"type": "text", "text": "src/server/routes.ts:34: const session = verifyToken(req.headers.authorization);\nsrc/auth/jwt.ts:12: export function verifyToken(token: string): TokenClaims {"}]}
    },
    {
        "tool": "postgres_query",
        "dur": 45,
        "offset": 180,
        "status": "ok",
        "args": {"query": "SELECT id, email, token_version, active FROM users WHERE active = true LIMIT 5;"},
        "res": {"content": [{"type": "text", "text": "[\n  {\"id\": \"usr_1\", \"email\": \"alex@acme.dev\", \"token_version\": 3, \"active\": true},\n  {\"id\": \"usr_2\", \"email\": \"sam@acme.dev\", \"token_version\": 1, \"active\": true}\n]"}]}
    },
    {
        "tool": "edit_file",
        "dur": 128,
        "offset": 280,
        "status": "ok",
        "args": {
            "path": "src/auth/jwt.ts",
            "patch": "--- a/src/auth/jwt.ts\n+++ b/src/auth/jwt.ts\n@@ -12,4 +12,8 @@\n export function verifyToken(token: string): TokenClaims {\n-  return jwt.verify(token, process.env.JWT_SECRET) as TokenClaims;\n+  const decoded = jwt.verify(token, process.env.JWT_SECRET) as TokenClaims;\n+  if (decoded.exp && decoded.exp < Math.floor(Date.now() / 1000)) {\n+    throw new TokenExpiredError('jwt expired');\n+  }\n+  return decoded;\n }"
        },
        "res": {"content": [{"type": "text", "text": "Applied 1 hunk successfully to src/auth/jwt.ts"}]}
    },
    {
        "tool": "shell",
        "dur": 480,
        "offset": 450,
        "status": "error",
        "args": {"command": "pnpm test tests/auth.test.ts"},
        "res": {"content": [{"type": "text", "text": "FAIL tests/auth.test.ts\n  ✓ verifies valid token (4ms)\n  ✕ rejects expired token (14ms)\n\n  AssertionError: expected TokenExpiredError to be thrown, but received undefined\n    at tests/auth.test.ts:42:11"}], "isError": True}
    },
    {
        "tool": "read_file",
        "dur": 16,
        "offset": 980,
        "status": "ok",
        "args": {"path": "src/server/routes.ts"},
        "res": {"content": [{"type": "text", "text": "router.post('/auth/refresh', async (req, res) => {\n  // refresh handler\n});"}]}
    },
    {
        "tool": "edit_file",
        "dur": 110,
        "offset": 1030,
        "status": "ok",
        "args": {
            "path": "src/server/routes.ts",
            "patch": "--- a/src/server/routes.ts\n+++ b/src/server/routes.ts\n@@ -34,3 +34,6 @@\n router.post('/auth/refresh', async (req, res) => {\n-  res.status(501).send();\n+  const { refreshToken } = req.body;\n+  const session = await rotateUserSession(refreshToken);\n+  res.json({ session });\n });"
        },
        "res": {"content": [{"type": "text", "text": "Applied 1 hunk successfully to src/server/routes.ts"}]}
    },
    {
        "tool": "redis_get",
        "dur": 8,
        "offset": 1180,
        "status": "ok",
        "args": {"query": "session:usr_99214:tokens"},
        "res": {"content": [{"type": "text", "text": "{\"token_version\": 3, \"last_rotated\": 1729088400, \"revoked\": false}"}]}
    },
    {
        "tool": "shell",
        "dur": 510,
        "offset": 1220,
        "status": "ok",
        "args": {"command": "pnpm test tests/auth.test.ts"},
        "res": {"content": [{"type": "text", "text": "PASS tests/auth.test.ts\n  ✓ verifies valid token (4ms)\n  ✓ rejects expired token (2ms)\n  ✓ refreshes session token (12ms)\n\nTest Suites: 1 passed, 1 total\nTests:       3 passed, 3 total\nTime:        0.49s"}]}
    },
    {
        "tool": "fetch",
        "dur": 24,
        "offset": 1780,
        "status": "ok",
        "args": {"query": "GET http://localhost:3000/api/health"},
        "res": {"content": [{"type": "text", "text": "HTTP/1.1 200 OK\nContent-Type: application/json\n\n{\"status\": \"healthy\", \"version\": \"2.4.0\", \"uptime\": 1482}"}]}
    },
    {
        "tool": "read_file",
        "dur": 9,
        "offset": 1840,
        "status": "ok",
        "args": {"path": "package.json"},
        "res": {"content": [{"type": "text", "text": "{\n  \"name\": \"acme-api\",\n  \"version\": \"2.4.0\"\n}"}]}
    },
    {
        "tool": "Grep",
        "dur": 29,
        "offset": 1880,
        "status": "ok",
        "args": {"query": "rotateUserSession src/", "path": "src/services/session.ts"},
        "res": {"content": [{"type": "text", "text": "src/services/session.ts:18: export async function rotateUserSession(refreshToken: string) {"}]}
    },
    {
        "tool": "postgres_query",
        "dur": 32,
        "offset": 1940,
        "status": "ok",
        "args": {"query": "UPDATE users SET token_version = token_version + 1 WHERE id = 'usr_1';"},
        "res": {"content": [{"type": "text", "text": "UPDATE 1"}]}
    },
    {
        "tool": "edit_file",
        "dur": 95,
        "offset": 2010,
        "status": "ok",
        "args": {
            "path": "tests/auth.test.ts",
            "patch": "--- a/tests/auth.test.ts\n+++ b/tests/auth.test.ts\n@@ -24,2 +24,6 @@\n   it('rotates expired session tokens', async () => {\n+    const session = await rotateUserSession(expiredToken);\n+    expect(session.active).toBe(true);\n   });"
        },
        "res": {"content": [{"type": "text", "text": "Applied 1 hunk successfully to tests/auth.test.ts"}]}
    },
    {
        "tool": "shell",
        "dur": 320,
        "offset": 2140,
        "status": "ok",
        "args": {"command": "pnpm lint"},
        "res": {"content": [{"type": "text", "text": "✔ No ESLint warnings or errors found."}]}
    },
    {
        "tool": "read_file",
        "dur": 11,
        "offset": 2500,
        "status": "ok",
        "args": {"path": "src/types/auth.d.ts"},
        "res": {"content": [{"type": "text", "text": "export interface SessionToken {\n  accessToken: string;\n  refreshToken: string;\n}"}]}
    },
    {
        "tool": "edit_file",
        "dur": 82,
        "offset": 2540,
        "status": "ok",
        "args": {
            "path": "src/types/auth.d.ts",
            "patch": "--- a/src/types/auth.d.ts\n+++ b/src/types/auth.d.ts\n@@ -3,2 +3,3 @@\n   refreshToken: string;\n+  expiresAt: number;\n }"
        },
        "res": {"content": [{"type": "text", "text": "Applied 1 hunk successfully to src/types/auth.d.ts"}]}
    },
    {
        "tool": "shell",
        "dur": 22,
        "offset": 2660,
        "status": "ok",
        "args": {"command": "git diff --stat"},
        "res": {"content": [{"type": "text", "text": " src/auth/jwt.ts       | 6 +++++-\n src/server/routes.ts  | 5 ++++-\n src/types/auth.d.ts   | 1 +\n tests/auth.test.ts    | 4 ++++\n 4 files changed, 16 insertions(+), 2 deletions(-)"}]}
    },
    {
        "tool": "shell",
        "dur": 14,
        "offset": 2710,
        "status": "ok",
        "args": {"command": "git status --porcelain"},
        "res": {"content": [{"type": "text", "text": " M src/auth/jwt.ts\n M src/server/routes.ts\n M src/types/auth.d.ts\n M tests/auth.test.ts"}]}
    },
    {
        "tool": "shell",
        "dur": 85,
        "offset": 2760,
        "status": "ok",
        "args": {"command": "git commit -m \"fix(auth): rotate expired session tokens on refresh\""},
        "res": {"content": [{"type": "text", "text": "[main 8c91fa0] fix(auth): rotate expired session tokens on refresh\n 4 files changed, 16 insertions(+), 2 deletions(-)"}]}
    },
    {
        "tool": "redis_get",
        "dur": 6,
        "offset": 2880,
        "status": "ok",
        "args": {"query": "session:cache:health"},
        "res": {"content": [{"type": "text", "text": "{\"status\": \"ready\", \"connections\": 14}"}]}
    },
    {
        "tool": "fetch",
        "dur": 38,
        "offset": 2920,
        "status": "ok",
        "args": {"query": "POST http://localhost:3000/api/auth/refresh"},
        "res": {"content": [{"type": "text", "text": "HTTP/1.1 200 OK\nContent-Type: application/json\n\n{\"session\": {\"accessToken\": \"eyJ...\", \"expiresAt\": 1729089300}}"}]}
    },
    {
        "tool": "shell",
        "dur": None, # OPEN / PENDING call!
        "offset": 3000,
        "status": "pending",
        "args": {"command": "pnpm build --watch"},
        "res": None
    }
]

lines = []
seq = 1

# SessionStarted
lines.append({
    "schemaVersion": 2,
    "recordingId": rec_id,
    "seq": seq,
    "sessionId": rec_id,
    "timestamp": base_time.isoformat(),
    "type": "SessionStarted",
    "risk": "none",
    "status": "ok"
})

for i, call in enumerate(calls_data, 1):
    seq += 1
    req_seq = seq
    call_id = f"{rec_id}/call/{i}"
    rpc_id = f"h{i}"
    start_ts = base_time + timedelta(milliseconds=call["offset"])
    
    # Request
    req_event = {
        "schemaVersion": 2,
        "recordingId": rec_id,
        "source": "cursor-hook",
        "sourceId": f"{rec_id}/cursor-hook",
        "agentSessionId": agent_session,
        "callId": call_id,
        "correlation": {"method": "fallback", "quality": "exact"},
        "capture": {
            "representation": "hook-normalized",
            "completeness": "complete",
            "observedBytes": len(json.dumps(call["args"])),
            "observedUnit": "json-value",
            "observedField": "tool_input",
            "normalizedBytes": len(json.dumps(call["args"])) + 80,
            "storedBytes": len(json.dumps(call["args"])) + 80,
            "profile": "standard",
            "changes": [{"kind": "normalized", "stage": "adapter", "method": "hook-normalization-v1"}]
        },
        "seq": req_seq,
        "sessionId": rec_id,
        "timestamp": start_ts.isoformat(),
        "type": "ToolCallRequest",
        "direction": "client->server",
        "method": "tools/call",
        "toolName": call["tool"],
        "rpcId": rpc_id,
        "risk": "none",
        "status": "pending",
        "payload": {
            "id": rpc_id,
            "jsonrpc": "2.0",
            "method": "tools/call",
            "params": {
                "name": call["tool"],
                "arguments": call["args"]
            }
        }
    }
    lines.append(req_event)
    
    if call["res"] is not None:
        seq += 1
        end_ts = start_ts + timedelta(milliseconds=call["dur"])
        res_event = {
            "schemaVersion": 2,
            "recordingId": rec_id,
            "source": "cursor-hook",
            "sourceId": f"{rec_id}/cursor-hook",
            "agentSessionId": agent_session,
            "callId": call_id,
            "correlation": {
                "method": "fallback",
                "quality": "exact",
                "requestSeq": req_seq
            },
            "timing": {
                "durationMs": call["dur"],
                "source": "source-reported"
            },
            "capture": {
                "representation": "hook-normalized",
                "completeness": "complete",
                "observedBytes": len(json.dumps(call["res"])),
                "observedUnit": "json-value",
                "observedField": "result_json",
                "normalizedBytes": len(json.dumps(call["res"])) + 60,
                "storedBytes": len(json.dumps(call["res"])) + 60,
                "profile": "standard",
                "changes": [{"kind": "normalized", "stage": "adapter", "method": "hook-normalization-v1"}]
            },
            "seq": seq,
            "sessionId": rec_id,
            "timestamp": end_ts.isoformat(),
            "type": "ToolCallResponse",
            "direction": "server->client",
            "method": "tools/call",
            "toolName": call["tool"],
            "rpcId": rpc_id,
            "risk": "none",
            "status": call["status"],
            "durationMs": call["dur"],
            "payload": {
                "result": call["res"]
            }
        }
        lines.append(res_event)

output_path = "/tmp/rich-diverse-session.jsonl"
with open(output_path, "w") as f:
    for item in lines:
        f.write(json.dumps(item) + "\n")
print(f"Generated {len(lines)} events to {output_path}")
