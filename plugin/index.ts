/**
 * KingCrab Plugin for OpenClaw
 * PAM for chat-based sudo approval workflows
 * Version: 1.0.0
 *
 * This plugin provides tools for:
 * - Creating elevation requests
 * - Listing pending requests
 * - Approving/denying requests
 */

import { Type } from "@sinclair/typebox";

// ============================================================================
// Types & Interfaces
// ============================================================================

interface PluginConfig {
  daemonUrl: string;
  timeout?: number;
  allowedCommands?: string[];
}

interface CreateRequestParams {
  command: string;
  reason?: string;
}

interface ListRequestsParams {
  status?: 'pending' | 'approved' | 'denied' | 'all';
}

interface DaemonRequest {
  command: string;
  reason?: string;
  requester?: string;
}

interface DaemonResponse {
  success: boolean;
  request?: {
    id: string;
    status: string;
    expires_at: string;
  };
  error?: string;
}

interface ListResponse {
  requests: Array<{
    id: string;
    command: string;
    reason: string;
    status: string;
    created_at: string;
    expires_at: string;
  }>;
  count: number;
}

// ============================================================================
// OpenClaw Plugin Entry Point
// ============================================================================

export default function (api: any) {
  // Get plugin configuration
  const cfg: PluginConfig = api.pluginConfig || {
    daemonUrl: 'http://localhost:8080'
  };

  const daemonUrl = cfg.daemonUrl;
  const timeout = cfg.timeout || 30000;

  // Get stable caller identity (from OpenClaw session or context)
  const getCallerIdentity = (): string => {
    // Try to get identity from API context
    if (api.session && api.session.user) {
      return `openclaw-user-${api.session.user.id}`;
    }
    if (api.context && api.context.agentId) {
      return `openclaw-agent-${api.context.agentId}`;
    }
    // Fallback
    return 'openclaw-agent';
  };

  /**
   * Send a JSON HTTP request to the configured KingCrab daemon API and return its response.
   *
   * Sends the request to `${daemonUrl}/api/v1${endpoint}` using either a Unix socket (when
   * `daemonUrl` starts with `unix:` or `/`) or standard HTTP/HTTPS transport. The request
   * uses `Content-Type: application/json` and honors the configured `timeout`.
   *
   * @param endpoint - Path appended to `/api/v1` (e.g. `/request` or `/request/:id`)
   * @param method - HTTP method to use; defaults to `GET`
   * @param body - Optional request payload serialized as JSON
   * @returns The daemon response parsed as JSON, or the raw response text when parsing fails
   * @throws Error with message `HTTP <status>: <message>` for non-2xx responses
   * @throws Error `Request timeout - daemon may be unavailable` when the request times out
   */
  async function daemonRequest(endpoint: string, method: string = 'GET', body?: any): Promise<any> {
    // Detect Unix socket transport
    const isUnixSocket = daemonUrl.startsWith('unix:') || daemonUrl.startsWith('/');

    if (isUnixSocket) {
      // Unix socket support - use Node.js http client
      const http = require('http');
      const socketPath = daemonUrl.startsWith('unix:') ? daemonUrl.slice(5) : daemonUrl;

      return new Promise((resolve, reject) => {
        const requestOptions = {
          socketPath: socketPath,
          path: `/api/v1${endpoint}`,
          method: method,
          headers: {
            'Content-Type': 'application/json',
          },
          timeout: timeout,
        };

        const req = http.request(requestOptions, (res: any) => {
          let data = '';
          res.on('data', (chunk: any) => { data += chunk; });
          res.on('end', () => {
            if (res.statusCode >= 200 && res.statusCode < 300) {
              try {
                resolve(JSON.parse(data));
              } catch {
                resolve(data);
              }
            } else {
              reject(new Error(`HTTP ${res.statusCode}: ${data || res.statusMessage}`));
            }
          });
        });

        req.on('error', (err: any) => reject(err));
        req.on('timeout', () => {
          req.destroy();
          reject(new Error('Request timeout - daemon may be unavailable'));
        });

        if (body) {
          req.write(JSON.stringify(body));
        }
        req.end();
      });
    }

    // Standard HTTP/HTTPS request
    const url = `${daemonUrl}/api/v1${endpoint}`;
    const options: RequestInit = {
      method,
      headers: {
        'Content-Type': 'application/json',
      },
    };

    if (body) {
      options.body = JSON.stringify(body);
    }

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    try {
      const response = await fetch(url, {
        ...options,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`HTTP ${response.status}: ${errorText || response.statusText}`);
      }

      return await response.json();
    } catch (error: any) {
      clearTimeout(timeoutId);
      if (error.name === 'AbortError') {
        throw new Error('Request timeout - daemon may be unavailable');
      }
      throw error;
    }
  }

  // ========================================================================
  // Tool: kingcrab_request
  // ========================================================================

  api.registerTool({
    name: 'kingcrab_request',
    label: 'Request Privileged Command',
    description: 'Submit a request for elevated command execution via KingCrab PAM. The request will be sent to Telegram for approval.',
    parameters: Type.Object({
      command: Type.String({ description: 'The sudo command to execute (e.g., "apt install golang-go")' }),
      reason: Type.Optional(Type.String({ description: 'Why this command is needed' })),
    }),
    async execute(_toolCallId: string, params: CreateRequestParams) {
      const { command, reason = '' } = params;

      try {
        const callerIdentity = getCallerIdentity();
        const result: DaemonResponse = await daemonRequest('/request', 'POST', {
          command,
          reason: reason || 'Requested via OpenClaw agent',
          requester: callerIdentity,
        });

        if (!result.success) {
          return {
            content: [{ type: 'text', text: `❌ Failed to create request: ${result.error}` }],
          };
        }

        const req = result.request;
        const shortId = req?.id.slice(0, 8) || 'unknown';
        const expires = req?.expires_at ? new Date(req.expires_at).toLocaleString() : 'unknown';

        return {
          content: [{
            type: 'text',
            text: `✅ KingCrab request ${shortId} created\n\nCommand: \`${command}\`\nExpires: ${expires}\n\nCheck Telegram to approve this request.`,
          }],
          details: result,
        };
      } catch (error: any) {
        return {
          content: [{ type: 'text', text: `❌ Error: ${error.message}` }],
        };
      }
    },
  });

  // ========================================================================
  // Tool: kingcrab_list
  // ========================================================================

  api.registerTool({
    name: 'kingcrab_list',
    label: 'List KingCrab Requests',
    description: 'List all pending or recent KingCrab elevation requests',
    parameters: Type.Object({
      status: Type.Optional(Type.Union([
        Type.Literal('pending'),
        Type.Literal('approved'),
        Type.Literal('denied'),
        Type.Literal('all'),
      ])),
    }),
    async execute(_toolCallId: string, params: ListRequestsParams) {
      const { status = 'pending' } = params;

      try {
        const queryParam = status !== 'all' ? `?status=${status}` : '';
        const result: ListResponse = await daemonRequest(`/requests${queryParam}`, 'GET');

        if (!result.requests || result.requests.length === 0) {
          return {
            content: [{ type: 'text', text: '📭 No requests found.' }],
          };
        }

        const pending = result.requests.filter(r => r.status === 'pending');
        const message = `📋 KingCrab Requests\n\nTotal: ${result.count}\nPending: ${pending.length}\n\n${
          result.requests
            .map((r) => {
              const shortId = r.id.slice(0, 8);
              const statusEmoji = r.status === 'pending' ? '⏳' : r.status === 'approved' ? '✅' : r.status === 'denied' ? '🚫' : '❓';
              return `${statusEmoji} ${shortId}: \`${r.command}\`\n   Reason: ${r.reason || 'N/A'}\n   Status: ${r.status}\n   Created: ${new Date(r.created_at).toLocaleString()}`;
            })
            .join('\n\n')
        }`;

        return {
          content: [{ type: 'text', text: message }],
        };
      } catch (error: any) {
        return {
          content: [{ type: 'text', text: `❌ Error: ${error.message}` }],
        };
      }
    },
  });

  // ========================================================================
  // Tool: kingcrab_approve (DISABLED)
  // ========================================================================
  // Note: Direct approval from the plugin is disabled for security reasons.
  // Approvals must be performed via biometric authentication through Telegram.
  // This tool registration is commented out to prevent client-side approvals.

  /*
  api.registerTool({
    name: 'kingcrab_approve',
    label: 'Approve KingCrab Request (DISABLED)',
    description: 'DISABLED: Approvals must be done via Telegram with biometric authentication',
    parameters: Type.Object({
      requestId: Type.String({ description: 'The request ID to approve' }),
    }),
    async execute(_toolCallId: string, params: { requestId: string }) {
      return {
        content: [{
          type: 'text',
          text: '❌ Direct approval is disabled. Please approve requests via Telegram with biometric authentication.',
        }],
      };
    },
  });
  */

  // ========================================================================
  // Tool: kingcrab_deny
  // ========================================================================

  api.registerTool({
    name: 'kingcrab_deny',
    label: 'Deny KingCrab Request',
    description: 'Deny a pending KingCrab elevation request',
    parameters: Type.Object({
      requestId: Type.String({ description: 'The request ID to deny' }),
      reason: Type.Optional(Type.String({ description: 'Reason for denial' })),
    }),
    async execute(_toolCallId: string, params: { requestId: string; reason?: string }) {
      const { requestId, reason = '' } = params;

      try {
        const result = await daemonRequest(`/request/${requestId}/deny`, 'POST', { reason });

        return {
          content: [{
            type: 'text',
            text: result.success
              ? `🚫 Request ${requestId.slice(0, 8)} denied.`
              : `❌ Failed to deny request: ${result.error || 'Unknown error'}`,
          }],
        };
      } catch (error: any) {
        return {
          content: [{ type: 'text', text: `❌ Error: ${error.message}` }],
        };
      }
    },
  });

  // ========================================================================
  // Tool: kingcrab_status
  // ========================================================================

  api.registerTool({
    name: 'kingcrab_status',
    label: 'Get KingCrab Status',
    description: 'Check the status of a specific KingCrab request',
    parameters: Type.Object({
      requestId: Type.String({ description: 'The request ID to check' }),
    }),
    async execute(_toolCallId: string, params: { requestId: string }) {
      const { requestId } = params;

      try {
        const result = await daemonRequest(`/request/${requestId}`, 'GET');

        const statusEmoji = result.status === 'pending' ? '⏳' : result.status === 'approved' ? '✅' : result.status === 'denied' ? '🚫' : result.status === 'completed' ? '✅' : result.status === 'failed' ? '❌' : '❓';

        return {
          content: [{
            type: 'text',
            text: `${statusEmoji} KingCrab Request ${requestId.slice(0, 8)}\n\nCommand: \`${result.command}\`\nReason: ${result.reason || 'N/A'}\nStatus: ${result.status}\nCreated: ${new Date(result.created_at).toLocaleString()}\nExpires: ${new Date(result.expires_at).toLocaleString()}${
              result.output ? `\n\nOutput:\n${result.output}` : ''
            }`,
          }],
        };
      } catch (error: any) {
        return {
          content: [{ type: 'text', text: `❌ Error: ${error.message}` }],
        };
      }
    },
  });
}