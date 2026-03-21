# KingCrab Documentation Structure

**Version:** 1.0.0
**Last Updated:** 2025-03-19

## Overview

This document defines the complete documentation structure for the KingCrab PAM system. All documentation files should follow these guidelines to ensure consistency and completeness.

---

## File Structure

```
kingcrab/
├── README.md                    # Project overview and quick start
├── INSTALL.md                   # Detailed installation guide
├── CONFIG.md                    # Configuration reference
├── API.md                       # Daemon API reference (link to API_SPECIFICATION.md)
├── ARCHITECTURE.md              # System architecture and design
├── SECURITY.md                  # Security model and threat analysis
├── TROUBLESHOOTING.md           # Common issues and solutions
├── CONTRIBUTING.md              # Contribution guidelines
├── CHANGELOG.md                 # Version history
├── LICENSE                      # License file
├── docs/
│   ├── API_SPECIFICATION.md     # REST API contract (detailed)
│   ├── PLUGIN_DEVELOPMENT.md    # Plugin development guide
│   ├── TELEGRAM_INTEGRATION.md  # Telegram bot setup and usage
│   └── openapi.yaml             # OpenAPI 3.0 spec
├── plugin/
│   ├── openclaw.plugin.json     # Plugin manifest
│   ├── README.md                # Plugin-specific documentation
│   ├── SKILL.md                 # Skill discovery document
│   └── config.example.json      # Plugin config template
├── skill/
│   ├── SKILL.md                 # Python skill documentation
│   └── skill.json               # Skill metadata
└── config/
    ├── config.example.json      # Daemon config template
    └── systemd/
        └── kingcrab.service     # systemd unit file
```

---

## File Descriptions

### Root Level Documentation

#### README.md
**Purpose:** Project overview, quick start, and basic usage

**Sections:**
1. Title and brief description
2. Why KingCrab (key benefits)
3. Architecture diagram
4. Quick start guide
5. Basic usage examples
6. Links to detailed docs
7. License and author

**Target Audience:** New users evaluating KingCrab

---

#### INSTALL.md
**Purpose:** Step-by-step installation instructions

**Sections:**
1. Prerequisites (Go, root access, Telegram bot)
2. Installation methods:
   - From source
   - From package (deb/rpm) - Future
   - Docker - Future
3. Configuration steps
4. Service setup (systemd)
5. Verification steps
6. Post-installation tasks
7. Uninstallation instructions

**Target Audience:** System administrators deploying KingCrab

---

#### CONFIG.md
**Purpose:** Complete configuration reference

**Sections:**
1. Configuration file locations
2. Daemon configuration (`/etc/kingcrab/config.json`)
3. Plugin configuration (`~/.openclaw/openclaw.json`)
4. Environment variables
5. Command-line flags
6. Configuration validation
7. Example configurations for common scenarios

**Target Audience:** System administrators and users customizing KingCrab

---

#### API.md
**Purpose:** Quick API reference (links to detailed spec)

**Sections:**
1. Quick reference table (endpoint, method, purpose)
2. Link to API_SPECIFICATION.md for full details
3. Authentication overview
4. Quick code examples
5. SDK/Client library links (future)

**Target Audience:** Developers integrating with KingCrab

---

#### ARCHITECTURE.md
**Purpose:** System design and architecture decisions

**Sections:**
1. High-level architecture
2. Component breakdown
3. Data flow diagrams
4. Communication protocols
5. Security boundaries
6. Scalability considerations
7. Technology choices and rationale

**Target Audience:** Architects and contributors

---

#### SECURITY.md
**Purpose:** Security model and threat analysis

**Sections:**
1. Security principles
2. Threat model
3. Attack surface analysis
4. Mitigation strategies
5. Audit logging
6. Compliance considerations
7. Security best practices
8. Reporting vulnerabilities

**Target Audience:** Security professionals and auditors

---

#### TROUBLESHOOTING.md
**Purpose:** Common issues and solutions

**Sections:**
1. Installation issues
2. Configuration problems
3. Runtime errors
4. Telegram bot issues
5. Plugin communication failures
6. Debug mode and logging
7. Getting help

**Target Audience:** All users

---

#### CONTRIBUTING.md
**Purpose:** Contribution guidelines

**Sections:**
1. How to contribute
2. Development setup
3. Code style guidelines
4. Pull request process
5. Testing requirements
6. Documentation standards
7. Code of conduct

**Target Audience:** Contributors

---

#### CHANGELOG.md
**Purpose:** Version history

**Format:** Keep a Changelog format
- Added
- Changed
- Deprecated
- Removed
- Fixed
- Security

**Target Audience:** All users

---

### docs/ Directory

#### API_SPECIFICATION.md
**Purpose:** Complete REST API contract

**Sections:**
1. Overview and architecture
2. Base URL and authentication
3. Endpoints (detailed)
4. Data types
5. Error handling
6. Rate limiting
7. Webhooks
8. Telegram integration
9. Security considerations
10. Versioning policy
11. OpenAPI reference

**Target Audience:** API consumers and integration developers

---

#### PLUGIN_DEVELOPMENT.md
**Purpose:** Plugin development guide

**Sections:**
1. OpenClaw plugin architecture
2. KingCrab plugin structure
3. Plugin manifest (`openclaw.plugin.json`)
4. Registering HTTP routes
5. Communicating with daemon
6. Skill development
7. Testing plugins
8. Debugging tips

**Target Audience:** Plugin developers

---

#### TELEGRAM_INTEGRATION.md
**Purpose:** Telegram bot setup and usage

**Sections:**
1. Creating a Telegram bot
2. Bot token configuration
3. Setting up webhooks
4. User enrollment (pairing)
5. Inline button handlers
6. Callback data format
7. Security considerations
8. Troubleshooting Telegram issues

**Target Audience:** Administrators setting up the approval bot

---

#### openapi.yaml
**Purpose:** Machine-readable API specification

**Content:** OpenAPI 3.0 specification for all daemon endpoints

**Usage:**
- Generate client SDKs
- API documentation tools
- Contract testing

---

### Plugin Directory

#### openclaw.plugin.json
**Purpose:** OpenClaw plugin manifest

**Required Fields:**
```json
{
  "name": "@khentertainment/kingcrab-plugin",
  "version": "1.0.0",
  "description": "...",
  "configSchema": { /* JSON Schema */ },
  "skills": [...],
  "httpRoutes": [...]
}
```

---

#### plugin/README.md
**Purpose:** Plugin-specific documentation

**Sections:**
1. Plugin description
2. Installation (as OpenClaw extension)
3. Configuration options
4. Exposed skills
5. HTTP endpoints
6. Development setup

---

#### plugin/SKILL.md
**Purpose:** OpenClaw skill discovery document

**Format:** Frontmatter with metadata + markdown content

**Required Metadata:**
```yaml
---
name: kingcrab
description: Privileged Access Management for OpenClaw
metadata:
  openclaw:
    requires:
      bins: []
      env: []
      config: ["kingcrab.enabled"]
---
```

---

#### plugin/config.example.json
**Purpose:** Plugin configuration template

**Content:** Example OpenClaw config with KingCrab section

---

### Config Directory

#### config.example.json
**Purpose:** Daemon configuration template

**Sections:**
1. All configuration options with comments
2. Default values
3. Examples for common scenarios

---

## Documentation Standards

### Writing Style
- Use clear, concise language
- Assume reader has basic technical knowledge
- Provide examples for complex concepts
- Use diagrams for architecture and flow
- Include code examples with syntax highlighting

### Formatting
- Use GitHub Flavored Markdown
- Include table of contents for long documents
- Use proper heading hierarchy (H1, H2, H3...)
- Include relative links between documents
- Use admonitions for important notes

### Code Examples
- Show language identifier in fence blocks
- Include comments explaining key parts
- Provide both request and response examples
- Show curl commands for API examples

### Diagrams
- Use ASCII art for simple diagrams
- Use Mermaid for flowcharts when supported
- Describe complex diagrams in text
- Include legend for symbols

### Versioning
- Mark document version in header
- Include last updated date
- Document breaking changes in CHANGELOG
- Keep API docs versioned

---

## Review Process

### Before Publishing
1. Technical accuracy review
2. Copy edit for clarity
3. Test all code examples
4. Verify all links work
5. Check formatting consistency

### Maintenance
- Review and update quarterly
- Update with each release
- solicit user feedback
- Track documentation issues separately

---

## Accessibility

### Language
- Primary: English
- Future translations: Spanish, Chinese, Japanese

### Formats
- Markdown (primary)
- PDF (generated for releases)
- HTML (auto-generated from Markdown)

---

## Template: New Documentation File

```markdown
# [Title]

**Version:** [version]
**Last Updated:** [YYYY-MM-DD]
**Status:** [Draft/Stable/Deprecated]

## Overview
[Brief description of what this document covers]

## Prerequisites
[What readers need to know before reading]

## [Main Content]
[Organize with clear headings]

## Examples
[Practical examples]

## See Also
[Links to related documents]

## Changelog
### [version] ([date])
- [Changes]
```

---

## Document Maintenance Assignments

| Document | Owner | Review Frequency |
|----------|-------|------------------|
| README.md | Project Lead | Per release |
| INSTALL.md | DevOps | Per release |
| CONFIG.md | Backend Lead | Per release |
| API_SPECIFICATION.md | API Lead | Per release |
| ARCHITECTURE.md | Architect | Quarterly |
| SECURITY.md | Security Lead | Quarterly |
| TROUBLESHOOTING.md | Support | As needed |

---

## Contributing Documentation

When contributing documentation:
1. Follow the structure defined in this document
2. Use the provided template
3. Test all examples
4. Update related documents
5. Update this file if adding new documentation

---

## Tools and Resources

### Recommended Tools
- Markdown linters: `markdownlint`, `vale`
- Diagram tools: Mermaid, PlantUML
- API docs: OpenAPI Generator
- Spell check: `cspell`

### Resources
- [Markdown Guide](https://www.markdownguide.org/)
- [OpenAPI Specification](https://spec.openapis.org/oas/v3.0.0)
- [Diataxis](https://diataxis.fr/) - Documentation framework

---

**Document Owner:** Documentation Team
**Last Reviewed:** 2025-03-19
**Next Review:** 2025-06-19
