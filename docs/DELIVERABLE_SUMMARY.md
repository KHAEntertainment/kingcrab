# KingCrab Documentation and API Specification Deliverable

**Date:** 2025-03-19
**Status:** Complete
**Version:** 1.0.0

---

## Executive Summary

This document summarizes the comprehensive documentation and API specification deliverable for the KingCrab PAM (Privileged Access Management) system integration with OpenClaw. All deliverables have been created and are ready for implementation.

---

## Deliverables Created

### 1. API Specification (docs/API_SPECIFICATION.md)

**File:** `/home/openclaw/projects/kingcrab/docs/API_SPECIFICATION.md`

**Contents:**
- Complete REST API contract between plugin and daemon
- Base URL and authentication specifications
- All endpoints documented with request/response schemas:
  - `GET /health` - Health check
  - `POST /request` - Create request
  - `GET /requests` - List requests
  - `GET /request/{id}` - Get request by ID
  - `POST /request/{id}/approve` - Approve request
  - `POST /request/{id}/deny` - Deny request
  - `DELETE /request/{id}` - Delete request (V1.1)
- Data type definitions
- Error handling documentation
- Rate limiting specifications (V1.1)
- Webhook notification format
- Telegram integration details
- Security considerations

**Key Design Decisions:**
- Current: No authentication (localhost only)
- V1: Shared secret Bearer token
- V1.1: Mutual TLS for production
- Callback data format: `kc_approve:<request_id>`, `kc_deny:<request_id>`

---

### 2. Documentation Structure (docs/DOCUMENTATION_STRUCTURE.md)

**File:** `/home/openclaw/projects/kingcrab/docs/DOCUMENTATION_STRUCTURE.md`

**Contents:**
- Complete file structure for all documentation
- Descriptions for each document's purpose and audience
- Documentation standards and formatting guidelines
- Review process and maintenance assignments
- Template for new documentation files
- Recommended tools and resources

**Document Structure Defined:**
```
kingcrab/
├── README.md                    # Project overview
├── INSTALL.md                   # Installation guide
├── CONFIG.md                    # Configuration reference
├── API.md                       # API quick reference
├── ARCHITECTURE.md              # System design
├── SECURITY.md                  # Security model
├── TROUBLESHOOTING.md           # Common issues
├── CONTRIBUTING.md              # Contribution guidelines
├── CHANGELOG.md                 # Version history
├── docs/
│   ├── API_SPECIFICATION.md     # ✅ Created
│   ├── DOCUMENTATION_STRUCTURE.md # ✅ Created
│   ├── DEEPWIKI_RESEARCH_QUERIES.md # ✅ Created
│   ├── PLUGIN_DEVELOPMENT.md    # To be created
│   ├── TELEGRAM_INTEGRATION.md  # To be created
│   └── openapi.yaml             # To be created
└── plugin/
    ├── openclaw.plugin.json     # ✅ Created
    ├── config.example.json      # ✅ Created
    └── SKILL.md                 # Needs update
```

---

### 3. DeepWiki Research Queries (docs/DEEPWIKI_RESEARCH_QUERIES.md)

**File:** `/home/openclaw/projects/kingcrab/docs/DEEPWIKI_RESEARCH_QUERIES.md`

**Contents:**
- 23 prepared research queries for OpenClaw repository
- 5 queries already executed with results documented
- 18 queries for implementation, testing, and V2 phases
- Query execution guidelines and templates
- Tracking table for query status

**Queries Executed:**
1. ✅ Telegram Inline Buttons and Callback Queries
2. ✅ Plugin Configuration Schema
3. ✅ Sending Messages to Telegram Users
4. ✅ Skill Metadata for Gating
5. ✅ Plugin HTTP Architecture

**Key Findings:**
- Buttons registered via `buttons` field in message payload
- Callback data limited to 64 characters
- Plugins define config via `openclaw.plugin.json` with JSON Schema
- Telegram auth controlled by `dmPolicy` and `allowFrom` config
- Skill gating via `metadata.openclaw.requires` in SKILL.md

---

### 4. Daemon Configuration Template (config/config.example.json)

**File:** `/home/openclaw/projects/kingcrab/config/config.example.json`

**Contents:**
- Complete daemon configuration with inline comments
- All configuration options documented
- Default values provided
- Security hardening options
- Telegram bot configuration
- Webhook notification settings
- Rate limiting configuration
- Audit logging options
- PAM integration settings (experimental)

**Key Configuration Sections:**
```json
{
  "version": "1.0.0",
  "listen": { "type": "unix", "path": "/var/run/kingcrab.sock" },
  "allowedCommands": [...],
  "requireReason": true,
  "telegram": { "botToken": "...", "allowedUsers": [...] },
  "audit": { "enabled": true, "file": "/var/log/kingcrab/audit.jsonl" },
  "rateLimit": { "enabled": true, ... },
  "webhook": { "enabled": false, "url": "..." }
}
```

---

### 5. Plugin Configuration Template (plugin/config.example.json)

**File:** `/home/openclaw/projects/kingcrab/plugin/config.example.json`

**Contents:**
- Complete plugin configuration for OpenClaw
- Plugin-specific settings
- Skill configuration overrides
- Environment variable options
- Notification preferences
- Webhook settings

**Key Configuration Sections:**
```json
{
  "plugins": {
    "entries": {
      "@khentertainment/kingcrab-plugin": {
        "enabled": true,
        "config": {
          "daemonUrl": "http://localhost:8080",
          "daemonToken": "...",
          "allowedCommands": [...]
        },
        "skills": {
          "entries": {
            "kingcrab": {
              "enabled": true,
              "config": {
                "showResults": true,
                "maxOutputLength": 2000
              }
            }
          }
        }
      }
    }
  }
}
```

---

### 6. Plugin Manifest (plugin/openclaw.plugin.json)

**File:** `/home/openclaw/projects/kingcrab/plugin/openclaw.plugin.json`

**Contents:**
- Complete OpenClaw plugin manifest
- JSON Schema for plugin configuration
- Skill definitions with config schemas
- HTTP route registrations
- Plugin metadata and requirements

**Key Sections:**
- `configSchema`: Full JSON Schema for validation
- `skills.entries.kingcrab`: Skill definition
- `httpRoutes`: Webhook and UI endpoints
- `requirements`: OpenClaw version compatibility

---

## API Contract Summary

### Endpoints

| Method | Endpoint | Purpose | Auth |
|--------|----------|---------|------|
| GET | `/health` | Health check | None |
| POST | `/request` | Create request | Bearer |
| GET | `/requests` | List requests | Bearer |
| GET | `/request/{id}` | Get request | Bearer |
| POST | `/request/{id}/approve` | Approve | Bearer/Telegram |
| POST | `/request/{id}/deny` | Deny | Bearer/Telegram |
| DELETE | `/request/{id}` | Delete | Bearer (admin) |

### Request/Response Example

**Create Request:**
```json
POST /request
{
  "command": "apt install golang-go",
  "reason": "Required for building CLI tools",
  "requestedBy": "agent-session-123",
  "telegramUserId": 123456789
}

Response 201:
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

### Authentication

**Current (V0):**
- No authentication
- Localhost only

**V1:**
- Shared secret Bearer token
- Header: `Authorization: Bearer <token>`

**V1.1:**
- Mutual TLS for production
- Rate limiting headers

---

## Configuration Overview

### Daemon (`/etc/kingcrab/config.json`)

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `listen.type` | string | "unix" | Listener type (unix/tcp) |
| `listen.path` | string | "/var/run/kingcrab.sock" | Unix socket path |
| `allowedCommands` | array | [...] | Command allowlist |
| `requireReason` | boolean | true | Require reason for requests |
| `telegram.botToken` | string | - | Telegram bot token |
| `telegram.allowedUsers` | array | [] | Allowed approver IDs |
| `audit.enabled` | boolean | true | Enable audit logging |
| `rateLimit.enabled` | boolean | true | Enable rate limiting |

### Plugin (`~/.openclaw/openclaw.json`)

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `daemonUrl` | string | "http://localhost:8080" | Daemon API URL |
| `daemonToken` | string | - | Auth token |
| `timeout` | integer | 30000 | Request timeout (ms) |
| `allowedCommands` | array | [...] | Command patterns |
| `notifications.enabled` | boolean | true | Send notifications |
| `webhookUrl` | string | - | Webhook URL |

---

## Telegram Integration

### Callback Data Format
```
kc_approve:<request_id>
kc_deny:<request_id>
```

### Message Format
```
🔐 KingCrab PAM Request

Command: apt install golang-go
Reason: Required for building CLI tools
Requested by: agent-session-123

[Approve] [Deny]
```

### Configuration
- Bot token from @BotFather
- User IDs from @userinfobot
- Enrollment via pairing (optional)

---

## OpenClaw Integration Points

### Plugin Manifest (`openclaw.plugin.json`)
- Defines plugin metadata
- Specifies config schema (JSON Schema)
- Declares skills
- Registers HTTP routes

### Skill Metadata (`SKILL.md`)
- Frontmatter with `metadata.openclaw.requires`
- Gating based on: bins, env, config, os
- Skill documentation in markdown

### HTTP Routes
- Registration via `api.registerHttpRoute(...)`
- Auth options: "gateway" or "plugin"
- Plugin routes processed before UI

### Telegram Integration
- Buttons via `sendMessage` with `buttons` array
- Callback handling via `callback_query` event
- Auth via `dmPolicy` and `allowFrom`

---

## Implementation Roadmap

### Phase 1: Core API (Current)
- [x] API specification
- [x] Configuration templates
- [x] Plugin manifest
- [ ] Daemon HTTP implementation
- [ ] Plugin HTTP client

### Phase 2: Telegram Integration
- [ ] Telegram bot setup
- [ ] Inline button handlers
- [ ] Callback query processing
- [ ] User enrollment

### Phase 3: Plugin Integration
- [ ] OpenClaw plugin implementation
- [ ] Skill implementation
- [ ] Webhook notifications
- [ ] UI for approval

### Phase 4: Production Hardening
- [ ] Authentication (Bearer token)
- [ ] Rate limiting
- [ ] Mutual TLS
- [ ] Audit logging
- [ ] Monitoring

---

## Next Steps

### Immediate (This Sprint)
1. Review and approve API specification
2. Implement daemon HTTP endpoints
3. Create plugin HTTP client
4. Test daemon-plugin communication

### Short Term (Next Sprint)
1. Implement Telegram bot integration
2. Update SKILL.md with proper metadata
3. Create plugin implementation
4. Write integration tests

### Medium Term
1. Production installer
2. Authentication implementation
3. Webhook notifications
4. Monitoring and observability

### Long Term
1. Mutual TLS
2. PAM module integration
3. Multi-database support
4. Advanced approval workflows

---

## File Locations

All deliverables are in the repository:

```
/home/openclaw/projects/kingcrab/
├── docs/
│   ├── API_SPECIFICATION.md          ✅ Complete
│   ├── DOCUMENTATION_STRUCTURE.md    ✅ Complete
│   └── DEEPWIKI_RESEARCH_QUERIES.md  ✅ Complete
├── config/
│   └── config.example.json           ✅ Complete
└── plugin/
    ├── openclaw.plugin.json          ✅ Complete
    └── config.example.json           ✅ Complete
```

---

## Questions for Review

Please review the following:

1. **API Design:**
   - Are the endpoint designs appropriate?
   - Is the authentication strategy (V1: Bearer token) acceptable?
   - Should we add additional endpoints?

2. **Configuration:**
   - Are all necessary configuration options included?
   - Are the defaults sensible?
   - Should we add environment variable overrides?

3. **Telegram Integration:**
   - Is the callback data format (`kc_approve:<id>`) appropriate?
   - Should we support multiple approval bots?

4. **Documentation:**
   - Is the documentation structure complete?
   - Are there missing documents?
   - Should we prioritize creating additional docs?

---

## Related Documents

- [API_SPECIFICATION.md](./API_SPECIFICATION.md) - Full REST API contract
- [DOCUMENTATION_STRUCTURE.md](./DOCUMENTATION_STRUCTURE.md) - Documentation guidelines
- [DEEPWIKI_RESEARCH_QUERIES.md](./DEEPWIKI_RESEARCH_QUERIES.md) - Research queries

---

**Document Owner:** Documentation Team
**Status:** Complete
**Last Updated:** 2025-03-19
