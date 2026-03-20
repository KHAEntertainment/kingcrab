#!/usr/bin/env python3
"""
KingCrab Skill for OpenClaw
PAM - Privileged Access Management via chat

SECURITY: Approve/Deny disabled via slash commands.
For POC: Uses Telegram polls - user votes to approve/deny.
"""

import json
import urllib.request
import urllib.error
import os
from typing import Optional

# For sending messages/polls via OpenClaw
try:
    from openclaw import message
except ImportError:
    message = None


class KingCrabSkill:
    """Skill for KingCrab PAM - chat-based sudo approval."""
    
    name = "kingcrab"
    description = "Privileged Access Management - request elevated commands with user approval"
    
    DAEMON_URL = "http://localhost:8080"
    
    # Store pending polls to check later
    pending_polls = {}  # poll_id -> request_id
    
    def __init__(self, daemon_url: str = None):
        self.daemon_url = daemon_url or os.getenv("KINGCRAB_DAEMON_URL", self.DAEMON_URL)

    def _api_request(self, method: str, endpoint: str, data: dict = None) -> dict:
        """Make API request to KingCrab daemon (v1 API)."""
        url = f"{self.daemon_url}/api/v1{endpoint}"
        headers = {"Content-Type": "application/json"}
        
        req_data = json.dumps(data).encode() if data else None
        
        req = urllib.request.Request(url, data=req_data, headers=headers, method=method)
        
        try:
            with urllib.request.urlopen(req, timeout=10) as response:
                return json.loads(response.read().decode())
        except urllib.error.HTTPError as e:
            error_body = e.read().decode()
            try:
                return json.loads(error_body)
            except:
                return {"error": f"HTTP {e.code}: {error_body}"}
        except urllib.error.URLError as e:
            return {"error": f"Connection failed: {e.reason}"}
    
    async def request(self, command: str, reason: str = "") -> str:
        """Submit a request for elevated command execution.
        
        Creates request AND sends a Telegram poll for approval.
        """
        if not command:
            return "Error: command is required"
        
        if not reason:
            reason = "No reason provided"
        
        # Create request on daemon
        result = self._api_request("POST", "/request", {
            "command": command,
            "reason": reason,
            "requester": "openclaw-skill"
        })
        
        if not result.get("success"):
            return f"❌ Failed to create request: {result.get('error', 'Unknown error')}"

        request_data = result.get("request", {})
        request_id = request_data.get("id", "")
        short_id = request_id[:8]
        
        # Return message indicating poll will be sent
        # The poll is sent by the skill system when response is displayed
        return (
            f"🔐 Request created: `{short_id}`\n"
            f"Command: `{command}`\n"
            f"Reason: {reason}\n\n"
            f"⏳ Please vote on the poll below to Approve/Deny"
        )
    
    async def list(self, status: str = None) -> str:
        """List all requests."""
        result = self._api_request("GET", "/requests")
        
        if "error" in result:
            return f"❌ Error: {result['error']}"
        
        requests = result if isinstance(result, list) else result.get("requests", [])
        
        if not requests:
            return "📭 No requests found"
        
        if status:
            requests = [r for r in requests if r.get("status") == status]
        
        if not requests:
            return f"📭 No {status or ''} requests"
        
        lines = [f"📋 Requests ({len(requests)})"]
        for req in requests:
            req_id = req.get("id", "")[:8]
            lines.append(f"`{req_id}` | {req.get('status')} | {req.get('command', '')[:40]}")
        
        return "\n".join(lines)
    
    async def status(self) -> str:
        """Check KingCrab daemon status."""
        result = self._api_request("GET", "/health")
        
        if "error" in result:
            return f"❌ Daemon unreachable: {result['error']}"
        
        return f"🦀 KingCrab: {result.get('status', 'unknown')} | Version: {result.get('version', '?')}"
    
    async def approve(self, request_id: str) -> str:
        """POC ONLY - direct approve for testing the daemon."""
        return "❌ Approvals disabled. Please use the poll to vote."
    
    async def deny(self, request_id: str) -> str:
        """POC ONLY - direct deny for testing."""
        return "❌ Denials disabled. Please use the poll to vote."
    
    async def check_polls(self) -> str:
        """Check poll results and process approvals.
        
        This would be called by cron/heartbeat to process poll votes.
        For POC, this is a placeholder - manual approval via daemon API for now.
        """
        return "⏳ Poll checking not yet implemented. Use /kc status to check manually."


# For direct testing
if __name__ == "__main__":
    import asyncio
    
    async def test():
        kc = KingCrabSkill()
        print(await kc.status())
        print(await kc.request("echo POC works", "testing"))
        print(await kc.list())
    
    asyncio.run(test())
