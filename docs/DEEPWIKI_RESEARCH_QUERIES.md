# DeepWiki Research Queries

**Version:** 1.0.0
**Last Updated:** 2025-03-19

## Overview

This document contains prepared DeepWiki queries for the OpenClaw repository (https://github.com/openclaw/openclaw). These queries should be executed during implementation to ensure accurate integration with OpenClaw's plugin architecture and Telegram integration patterns.

---

## Queries Already Executed (2025-03-19)

### ✅ 1. Telegram Inline Buttons and Callback Queries

**Query:** "How do plugins register Telegram inline buttons and handle callback queries? What are the interfaces and patterns for interactive Telegram components?"

**Key Findings:**
- Buttons registered via `buttons` field in message payload
- Callback data limited to 64 characters
- Handled by `callback_query` event listener in bot
- Controlled by `channels.telegram.capabilities.inlineButtons` config
- Can be set to: `off`, `dm`, `group`, `all`, `allowlist`

**Relevance:** High - Critical for V2 approval bot implementation

---

### ✅ 2. Plugin Configuration Schema

**Query:** "What is the plugin configuration schema? How do plugins define their config structure, environment variables, and binary dependencies?"

**Key Findings:**
- Plugins define config via `openclaw.plugin.json`
- Must include `configSchema` with JSON Schema
- Env vars injected via `plugins.entries.*.env`
- Binary dependencies specified via config fields (e.g., `command` path)

**Relevance:** High - Required for plugin manifest

---

### ✅ 3. Sending Messages to Telegram Users

**Query:** "How do plugins send messages to specific Telegram users? What are the authentication and permission models?"

**Key Findings:**
- Use `telegramPlugin.outbound` interface
- Methods: `sendText`, `sendMedia`, `sendPayload`
- Auth controlled by `dmPolicy` and `allowFrom` config
- Options: `pairing`, `allowlist`, `open`, `disabled`

**Relevance:** High - Required for approval notifications

---

### ✅ 4. Skill Metadata for Gating

**Query:** "What is the skill metadata format for gating requirements including bins, env vars, and config checks?"

**Key Findings:**
- Defined in `metadata.openclaw.requires` in SKILL.md
- Supports: `bins`, `anyBins`, `env`, `config`, `os`
- Evaluated by `evaluateRequirements` function

**Relevance:** High - Required for SKILL.md

---

### ✅ 5. Plugin HTTP Architecture

**Query:** "How does the daemon-service plugin architecture work? What are the HTTP endpoints, authentication mechanisms, and communication patterns?"

**Key Findings:**
- Plugins register routes via `api.registerHttpRoute(...)`
- Auth options: `"gateway"` or `"plugin"`
- Plugin routes processed before Control UI
- One-way loading: plugin → registry → core

**Relevance:** High - Required for plugin HTTP endpoints

---

## Queries for Implementation Phase

### 📋 6. Telegram Bot Token Configuration

**Query:** "How do plugins securely store and access Telegram bot tokens? What are the best practices for credential management in plugin configs?"

**When to Run:** Before implementing Telegram bot integration

**Expected Output:**
- Credential storage patterns
- Environment variable injection
- Config field encryption (if any)

---

### 📋 7. Telegram User Enrollment/Pairing

**Query:** "How does the OpenClaw Telegram pairing system work? How do plugins enroll new Telegram users for approval workflows?"

**When to Run:** Before implementing user enrollment

**Expected Output:**
- Pairing flow and commands
- `allowFrom` management
- User ID retrieval methods

---

### 📋 8. Plugin Hook System

**Query:** "What hooks are available for plugins? Can plugins register lifecycle hooks or event handlers for daemon state changes?"

**When to Run:** Before implementing webhook notifications

**Expected Output:**
- Available hook types
- Hook registration patterns
- Event payload structures

---

### 📋 9. Skill Command Parameters

**Query:** "How do skills define complex command parameters with validation? What parameter types and validators are supported?"

**When to Run:** Before finalizing skill command schema

**Expected Output:**
- Parameter type definitions
- Validation patterns
- Auto-generated help text

---

### 📋 10. Plugin-to-Plugin Communication

**Query:** "Can plugins communicate with each other? How would the KingCrab plugin access the Telegram plugin's outbound interface?"

**When to Run:** Before implementing Telegram notifications from plugin

**Expected Output:**
- Inter-plugin communication patterns
- Dependency injection
- API access methods

---

### 📋 11. OpenClaw Logging Integration

**Query:** "How do plugins integrate with OpenClaw's logging system? What log levels and formats are recommended?"

**When to Run:** Before implementing structured logging

**Expected Output:**
- Logger access patterns
- Log level configuration
- Structured logging best practices

---

### 📋 12. Error Handling and User Feedback

**Query:** "How should plugins report errors to users? What are the patterns for error messages in skill responses?"

**When to Run:** Before implementing error handling

**Expected Output:**
- Error response formats
- User notification patterns
- Error recovery strategies

---

### 📋 13. Plugin Update/Reload Behavior

**Query:** "What happens when a plugin is updated or reloaded? Are there cleanup procedures or state migration considerations?"

**When to Run:** Before handling plugin lifecycle

**Expected Output:**
- Reload triggers
- State persistence
- Cleanup procedures

---

### 📋 14. Rate Limiting in OpenClaw

**Query:** "Does OpenClaw provide rate limiting for skills or plugin endpoints? How should plugins implement their own rate limiting?"

**When to Run:** Before implementing rate limiting

**Expected Output:**
- Built-in rate limiting (if any)
- Recommended rate limiting strategies
- Per-user vs per-key limits

---

### 📋 15. Skill Discovery and Registration

**Query:** "How does OpenClaw discover and register skills from plugins? What are the lifecycle events for skill loading?"

**When to Run:** Before finalizing skill packaging

**Expected Output:**
- Skill discovery process
- Registration endpoints
- Lazy loading options

---

## Queries for Testing Phase

### 🧪 16. Testing Plugin HTTP Routes

**Query:** "How do you test plugin HTTP routes in development? Are there test utilities or mock servers?"

**When to Run:** Before writing integration tests

**Expected Output:**
- Test environment setup
- Mock server options
- Test authentication methods

---

### 🧪 17. Testing Telegram Integration

**Query:** "How do you test Telegram bot integration locally? Are there testing utilities or webhook simulators?"

**When to Run:** Before writing Telegram tests

**Expected Output:**
- Local testing setup
- Webhook testing tools
- Bot API mocking

---

### 🧪 18. E2E Testing for Skills

**Query:** "What are the recommended approaches for end-to-end testing of OpenClaw skills?"

**When to Run:** Before writing skill tests

**Expected Output:**
- E2E test patterns
- Test data setup
- Assertion strategies

---

## Queries for V2 Features

### 🔮 19. Custom Telegram Bot Instances

**Query:** "Can plugins create their own Telegram bot instances, or must they use the shared OpenClaw bot?"

**When to Run:** When considering dedicated approval bot

**Expected Output:**
- Multi-bot support
- Bot registration patterns
- Resource allocation

---

### 🔮 20. Webhook Registration and Management

**Query:** "How do plugins register webhooks with external services? Are there utilities for webhook validation and retry logic?"

**When to Run:** When implementing webhook notifications

**Expected Output:**
- Webhook registration patterns
- Retry mechanisms
- Signature validation

---

### 🔮 21. Persistent Storage for Plugins

**Query:** "Do plugins have access to persistent storage? What are the recommended patterns for storing request history?"

**When to Run:** When implementing audit log persistence

**Expected Output:**
- Storage options
- Data directory patterns
- Persistence best practices

---

### 🔮 22. Background Services in Plugins

**Query:** "Can plugins run background services or scheduled tasks? How are long-running processes managed?"

**When to Run:** When implementing request cleanup/expiration

**Expected Output:**
- Background service patterns
- Scheduling options
- Process lifecycle

---

### 🔮 23. Plugin Dependencies and Versioning

**Query:** "How do plugins declare dependencies on OpenClaw versions or other plugins? How are version conflicts resolved?"

**When to Run:** When preparing for distribution

**Expected Output:**
- Version declaration
- Dependency resolution
- Compatibility checking

---

## Query Execution Guidelines

### Before Running Queries
1. Check if the information has changed since last research
2. Review OpenClaw changelog for relevant updates
3. Identify specific files or modules the query targets

### After Running Queries
1. Document key findings in relevant project docs
2. Update implementation plans based on new information
3. Share critical discoveries with the team
4. Update this document with executed query results

### Query Priority
- **High (📋):** Required for current implementation
- **Medium (🧪):** Required for testing
- **Low (🔮):** Future feature research

---

## Template for New Queries

```markdown
### 📋 [Number]. [Title]

**Query:** "[Specific question about OpenClaw]"

**When to Run:** [Development phase or trigger]

**Expected Output:**
- [Key information expected]
- [Relevant files or modules]

**Relevance:** High/Medium/Low - [Why this matters]

**Status:** Pending/Executed

**Result:** [Summary of findings after execution]
**Date:** [YYYY-MM-DD]
```

---

## Query Tracking

| Number | Title | Priority | Status | Date |
|--------|-------|----------|--------|------|
| 1 | Telegram Inline Buttons | High | ✅ Executed | 2025-03-19 |
| 2 | Plugin Config Schema | High | ✅ Executed | 2025-03-19 |
| 3 | Sending Messages | High | ✅ Executed | 2025-03-19 |
| 4 | Skill Metadata | High | ✅ Executed | 2025-03-19 |
| 5 | HTTP Architecture | High | ✅ Executed | 2025-03-19 |
| 6-15 | Implementation Queries | High | 📋 Pending | - |
| 16-18 | Testing Queries | Medium | 🧪 Pending | - |
| 19-23 | V2 Feature Queries | Low | 🔮 Pending | - |

---

## Related Documents

- [API_SPECIFICATION.md](./API_SPECIFICATION.md) - REST API contract
- [DOCUMENTATION_STRUCTURE.md](./DOCUMENTATION_STRUCTURE.md) - Documentation guidelines
- [PLUGIN_DEVELOPMENT.md](./PLUGIN_DEVELOPMENT.md) - Plugin implementation guide

---

**Document Owner:** Research Team
**Last Updated:** 2025-03-19
**Next Review:** As queries are executed
