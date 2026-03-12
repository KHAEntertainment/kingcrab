# KingCrab Skill (Native)

Direct skill for KingCrab PAM without plugin overhead.

## Usage

### Request elevated access
```
/kc request "sudo apt install golang-go" --reason "Need Go for building the CLI"
```

### List pending requests
```
/kc list
```

### Approve request
```
/kc approve <id>
```

### Deny request
```
/kc deny <id>
```

## Internal Commands

These are handled by the skill internally:

- `kingcrab.request` — Creates a new request via daemon API
- `kingcrab.list` — Lists all requests
- `kingcrab.approve` — Approves by ID
- `kingcrab.deny` — Denies by ID

## Daemon API

The skill communicates with KingCrab daemon at `http://localhost:8080`:

```
POST /request     — Create request
GET  /requests    — List all
POST /approve/:id — Approve
POST /deny/:id    — Deny
```
