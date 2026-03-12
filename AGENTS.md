# Project: KingCrab

**What:** Privileged Access Management (PAM) for OpenClaw — chat-based sudo approval  
**Why:** Enable agents to request elevated permissions without requiring direct sudo access  
**Done when:** MVP daemon runs with approve/deny workflow and audit logging

## Current Status
- **State:** 🟢 active
- **Last worked:** 2026-03-05
- **Assigned:** Barry
- **Next action:** Implement Unix socket support and persistent request queue

## Quick Links
- tasks.md — development checklist
- notes.md — technical decisions and research
- README.md — full project context
- internal/daemon/ — Go daemon code

---

## Phase Progress

### Phase 1: Daemon Core (Complete)
- [x] Go module initialized
- [x] HTTP server with health endpoint
- [x] Request queue (in-memory)
- [x] Basic JSON logging

### Phase 2: Production Ready (Current)
- [ ] Unix socket support
- [ ] Persistent queue (SQLite)
- [ ] Audit log to file
- [ ] Request expiration/timeout
- [ ] Systemd service

### Phase 3: Integration
- [ ] OpenClaw plugin
- [ ] ClawHub skill
- [ ] NPM package
