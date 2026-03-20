# KingCrab API Specification

**Version:** 1.0.0
**Last Updated:** 2025-03-19
**Status:** Draft

## Overview

The KingCrab API defines the REST contract between the OpenClaw plugin and the KingCrab daemon. This API enables privileged access management through a request-approval workflow.

## Architecture

```
┌─────────────────┐                    ┌──────────────────┐
│  OpenClaw       │   HTTP/REST        │  KingCrab        │
│  Plugin         │ ◄─────────────────► │  Daemon (Go)     │
│  (TypeScript)   │   localhost:8080    │  (root service)  │
└─────────────────┘                    └──────────────────┘
                                                │
                                                │ Telegram Bot API
                                                ▼
                                        ┌──────────────────┐
                                        │  Approval Bot    │
                                        │  (2FA Interface) │
                                        └──────────────────┘
```

## Base URL

- **Development:** `http://localhost:8080`
- **Production:** Unix socket at `/var/run/kingcrab.sock` (via HTTP-over-unix)

## Authentication

### Plugin-to-Daemon Authentication

**Current Status:** None (localhost only)

**Planned V1:**
- **Method:** Shared secret token in `Authorization` header
- **Header Format:** `Authorization: Bearer <shared_secret>`
- **Configuration:**
  - Daemon: `/etc/kingcrab/config.json` → `auth.pluginToken`
  - Plugin: `openclaw.json` → `kingcrab.daemonToken`

**Example:**
```http
POST /request HTTP/1.1
Host: localhost:8080
Authorization: Bearer kc_shared_secret_abc123
Content-Type: application/json
```

### Telegram Webhook Authentication

**Method:** Header-based secret token validation

- Telegram sends the `X-Telegram-Bot-Api-Secret-Token` HTTP header on webhook delivery
- The webhook handler in `webhook.go` validates this header value against the configured `telegram.botToken`
- If the header does not match the configured token, the request is rejected with HTTP 401 Unauthorized

## Endpoints

### 1. Health Check

Check daemon health and version.

**Endpoint:** `GET /api/v1/health`

**Authentication:** None

**Request:**
```http
GET /api/v1/health HTTP/1.1
```

**Response (200 OK):**
```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

---

### 2. Create Request

Submit a new privileged command execution request.

**Endpoint:** `POST /api/v1/request`

**Authentication:** Required (Bearer token)

**Request Headers:**
```http
Content-Type: application/json
Authorization: Bearer <token>
```

**Request Body:**
```json
{
  "command": "apt install golang-go",
  "reason": "Required for building CLI tools",
  "requestedBy": "agent-session-123",
  "telegramUserId": 123456789
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `command` | string | Yes | The command to execute (will be validated against allowlist) |
| `reason` | string | Conditional | Required if `requireReason` is enabled in daemon config |
| `requestedBy` | string | No | Identifier for who/what is making the request |
| `telegramUserId` | integer | No | Telegram user ID to notify for approval |

**Response (201 Created):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "command": "apt install golang-go",
  "reason": "Required for building CLI tools",
  "requestedBy": "agent-session-123",
  "status": "pending",
  "timestamp": "2025-03-19T10:30:00Z",
  "result": null
}
```

**Error Responses:**

| Code | Description | Body |
|------|-------------|------|
| 400 | Missing/invalid fields | `{"error": "command is required"}` |
| 403 | Command not in allowlist | `{"error": "command not allowed: ..."}` |
| 400 | Reason required but missing | `{"error": "reason is required"}` |
| 401 | Invalid authentication | `{"error": "unauthorized"}` |

---

### 3. List Requests

Retrieve all requests, optionally filtered by status.

**Endpoint:** `GET /api/v1/requests`

**Query Parameters:**
- `status` (optional): Filter by status (`pending`, `approved`, `denied`, `completed`, `failed`)
- `limit` (optional): Maximum number of requests to return (default: 100)
- `offset` (optional): Pagination offset (default: 0)

**Authentication:** Required (Bearer token)

**Request:**
```http
GET /api/v1/requests?status=pending&limit=10 HTTP/1.1
Authorization: Bearer <token>
```

**Response (200 OK):**
```json
{
  "requests": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "command": "apt install golang-go",
      "reason": "Required for building CLI tools",
      "requestedBy": "agent-session-123",
      "status": "pending",
      "timestamp": "2025-03-19T10:30:00Z",
      "result": null
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

---

### 4. Get Request by ID

Retrieve details of a specific request.

**Endpoint:** `GET /api/v1/request/{id}`

**Authentication:** Required (Bearer token)

**Request:**
```http
GET /api/v1/request/550e8400-e29b-41d4-a716-446655440000 HTTP/1.1
Authorization: Bearer <token>
```

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "command": "apt install golang-go",
  "reason": "Required for building CLI tools",
  "requestedBy": "agent-session-123",
  "status": "pending",
  "timestamp": "2025-03-19T10:30:00Z",
  "result": null
}
```

**Error Responses:**
- 404: Request not found

---

### 5. Approve Request

Approve a pending request and execute the command.

**Endpoint:** `POST /api/v1/request/{id}/approve`

**Authentication:** Required (Bearer token) OR Telegram webhook

**Request Headers (Plugin):**
```http
Authorization: Bearer <token>
```

**Request Body (Optional):**
```json
{
  "approvedBy": "user-telegram-123",
  "approvedAt": "2025-03-19T10:35:00Z"
}
```

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "command": "apt install golang-go",
  "reason": "Required for building CLI tools",
  "requestedBy": "agent-session-123",
  "status": "completed",
  "timestamp": "2025-03-19T10:30:00Z",
  "result": {
    "exitCode": 0,
    "stdout": "...",
    "stderr": "...",
    "duration": 2345
  }
}
```

**Error Responses:**
- 404: Request not found
- 400: Request not in pending state
- 500: Command execution failed (status will be `failed`)

---

### 6. Deny Request

Deny a pending request.

**Endpoint:** `POST /api/v1/request/{id}/deny`

**Authentication:** Required (Bearer token) OR Telegram webhook

**Request Headers (Plugin):**
```http
Authorization: Bearer <token>
```

**Request Body (Optional):**
```json
{
  "deniedBy": "user-telegram-123",
  "deniedReason": "Not approved at this time"
}
```

**Response (200 OK):**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "command": "apt install golang-go",
  "reason": "Required for building CLI tools",
  "requestedBy": "agent-session-123",
  "status": "denied",
  "timestamp": "2025-03-19T10:30:00Z",
  "result": null
}
```

---

### 7. Delete Request (V1.1)

Delete a request from the queue (admin only).

**Endpoint:** `DELETE /api/v1/request/{id}`

**Authentication:** Required (Bearer token with admin scope)

**Response (204 No Content):**

---

## Data Types

### Request Status

| Value | Description |
|-------|-------------|
| `pending` | Request created, awaiting approval |
| `approved` | Request approved, execution in progress |
| `denied` | Request denied by approver |
| `completed` | Command executed successfully |
| `failed` | Command execution failed |

### Request Object

```typescript
interface Request {
  id: string;              // UUID v4
  command: string;         // Command to execute
  reason: string;          // Justification for request
  requestedBy: string;     // Requestor identifier
  status: RequestStatus;
  timestamp: string;       // ISO 8601 datetime
  result: ExecuteResult | null;
  approvedBy?: string;     // Approver identifier (if approved/denied)
  deniedBy?: string;       // Denier identifier (if denied)
}
```

### ExecuteResult

```typescript
interface ExecuteResult {
  exitCode: number;        // Command exit code
  stdout: string;          // Standard output
  stderr: string;          // Standard error
  duration: number;        // Execution time in milliseconds
}
```

---

## Error Handling

All error responses follow this format:

```json
{
  "error": "Error message",
  "code": "ERROR_CODE",
  "details": {}
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `UNAUTHORIZED` | 401 | Invalid or missing authentication |
| `FORBIDDEN` | 403 | Command not in allowlist |
| `NOT_FOUND` | 404 | Request not found |
| `INVALID_REQUEST` | 400 | Malformed request |
| `ALREADY_PROCESSED` | 400 | Request already approved/denied |
| `EXECUTION_FAILED` | 500 | Command execution error |

---

## Rate Limiting (V1.1)

To prevent abuse, the following rate limits will apply:

| Scope | Limit | Window |
|-------|-------|--------|
| Plugin IP | 100 requests | 1 minute |
| Per-agent session | 10 create requests | 1 minute |
| Per-Telegram user | 30 approvals | 1 hour |

Rate limit headers:
```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1647700000
```

---

## Webhooks (V1.1)

The daemon can notify the plugin of request status changes.

### Configuration

Daemon config (`/etc/kingcrab/config.json`):
```json
{
  "webhook": {
    "url": "http://localhost:3000/kingcrab/webhook",
    "secret": "webhook_shared_secret",
    "events": ["request.completed", "request.failed"]
  }
}
```

### Webhook Payload

```json
{
  "event": "request.completed",
  "timestamp": "2025-03-19T10:35:00Z",
  "request": { /* Request object */ }
}
```

---

## Telegram Integration

### Callback Data Format

Inline button callbacks use this format:

```
kc_approve_<request_id>
deny_<request_id>
```

### Message Format

When a request is created, the bot sends:

```
🔐 KingCrab PAM Request

Command: apt install golang-go
Reason: Required for building CLI tools
Requested by: agent-session-123

[Approve] [Deny]
```

### Webhook Handler

**Endpoint:** `POST /api/v1/telegram/webhook`

**Authentication:** Telegram token validation via `X-Telegram-Bot-Api-Secret-Token` header

**Request Body:** Telegram Update object (as per Bot API)

---

## Security Considerations

### Plugin Communication
- **Current:** Localhost only, no auth
- **V1:** Shared secret Bearer token
- **V1.1:** Mutual TLS for production environments

### Command Validation
- All commands validated against allowlist before queueing
- Wildcard matching using regex patterns
- Shell metacharacter escaping

### Audit Logging
- All requests logged with:
  - Request ID
  - Command and reason
  - Requestor and approver
  - Timestamps
  - Execution result

### CSRF Protection
- State parameter for approve/deny callbacks
- Short-lived approval tokens (V1.1)

---

## Versioning

API versioning via URL path:
- Current: `/v1/*`
- Future: `/v2/*`

Breaking changes will increment major version.

---

## OpenAPI Specification

A complete OpenAPI 3.0 spec is available at:
`/docs/openapi.yaml` (served by daemon)

---

## Changelog

### 1.0.0 (2025-03-19)
- Initial API specification
- Basic CRUD operations for requests
- Telegram webhook support
- Plugin authentication planned

### 1.1.0 (Planned)
- Rate limiting
- Webhook notifications
- Request deletion
- Approval tokens
- Request metadata/labels

---

## Support

For issues or questions:
- GitHub: https://github.com/KHAEntertainment/kingcrab
- Documentation: https://kingcrab.khaintertainment.com