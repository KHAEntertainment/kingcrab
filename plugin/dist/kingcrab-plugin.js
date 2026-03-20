#!/usr/bin/env -S node
/**
 * KingCrab Plugin for OpenClaw
 * PAM for chat-based sudo approval workflows
 * Version: 0.1.0
 */
import { Plugin, HTTPClient } from './plugin-core.js';
import { z } from 'zod';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';
// Plugin config schema
const KingCrabConfigSchema = z.object({
    daemonUrl: z.string().url().default('http://localhost:8080'),
    allowedCommands: z.array(z.string()).default([
        'apt install *',
        'apt update',
        'systemctl restart *',
        'systemctl start *',
        'systemctl stop *',
        'systemctl status *',
    ]),
});
// ============================================================================
// Plugin Implementation
// ============================================================================
class KingCrabPlugin extends Plugin {
    config;
    httpClient;
    baseUrl;
    constructor() {
        super({
            name: 'kingcrab',
            version: '0.1.0',
            description: 'PAM for chat-based sudo approval workflows',
            author: 'KHAEntertainment',
        });
    }
    async onLoad(config) {
        // Validate and parse configuration
        const parsed = KingCrabConfigSchema.parse(config);
        this.config = parsed;
        // Initialize HTTP client for daemon communication
        this.httpClient = new HTTPClient({
            baseURL: parsed.daemonUrl,
            timeout: 5000,
        });
        this.baseUrl = parsed.daemonUrl;
        this.logger.info('KingCrab plugin loaded', {
            daemonUrl: this.baseUrl,
            allowedCommandsCount: parsed.allowedCommands.length,
        });
    }
    async onUnload() {
        this.logger.info('KingCrab plugin unloaded');
    }
    // ==========================================================================
    // HTTP API Endpoints (for agents to call)
    // ==========================================================================
    getRoutes() {
        return [
            {
                method: 'POST',
                path: '/kingcrab/request',
                handler: this.createRequest.bind(this),
            },
            {
                method: 'GET',
                path: '/kingcrab/requests',
                handler: this.listRequests.bind(this),
            },
            {
                method: 'POST',
                path: '/kingcrab/approve/:id',
                handler: this.approveRequest.bind(this),
            },
            {
                method: 'POST',
                path: '/kingcrab/deny/:id',
                handler: this.denyRequest.bind(this),
            },
        ];
    }
    // POST /kingcrab/request - Create a new privileged command request
    async createRequest(req, res) {
        try {
            const { command, reason } = req.body;
            if (!command) {
                res.status(400).json({
                    success: false,
                    error: 'Command is required',
                });
                return;
            }
            // Check against allowlist
            const isAllowed = this.config.allowedCommands.some((pattern) => {
                // Escape regex special characters except *, then convert * to .*
                const escaped = pattern.replace(/[.+?^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*');
                const regex = new RegExp('^' + escaped + '$');
                return regex.test(command);
            });
            if (!isAllowed) {
                res.status(403).json({
                    success: false,
                    error: `Command not in allowlist: ${command}`,
                });
                return;
            }
            // Forward to daemon
            const response = await this.httpClient.post('/request', {
                command,
                reason: reason || 'No reason provided',
            });
            if (!response.success) {
                res.status(400).json(response);
                return;
            }
            res.json({
                success: true,
                request: response.request,
                message: 'Request created successfully. Use inline buttons to approve/deny.',
            });
        }
        catch (error) {
            this.logger.error('Failed to create request', { error: error.message });
            res.status(500).json({
                success: false,
                error: 'Failed to create request',
            });
        }
    }
    // GET /kingcrab/requests - List all requests
    async listRequests(req, res) {
        try {
            const response = await this.httpClient.get('/requests');
            res.json({
                success: true,
                requests: response.requests,
            });
        }
        catch (error) {
            this.logger.error('Failed to list requests', { error: error.message });
            res.status(500).json({
                success: false,
                error: 'Failed to list requests',
            });
        }
    }
    // POST /kingcrab/approve/:id - Approve a request
    async approveRequest(req, res) {
        try {
            const { id } = req.params;
            const response = await this.httpClient.post(`/approve/${id}`, {});
            res.json(response);
        }
        catch (error) {
            this.logger.error('Failed to approve request', { error: error.message });
            res.status(500).json({
                success: false,
                error: 'Failed to approve request',
            });
        }
    }
    // POST /kingcrab/deny/:id - Deny a request
    async denyRequest(req, res) {
        try {
            const { id } = req.params;
            const response = await this.httpClient.post(`/deny/${id}`, {});
            res.json(response);
        }
        catch (error) {
            this.logger.error('Failed to deny request', { error: error.message });
            res.status(500).json({
                success: false,
                error: 'Failed to deny request',
            });
        }
    }
    // ==========================================================================
    // Inline Button Callbacks (for Telegram approve/deny)
    // ==========================================================================
    async handleInlineCallback(callbackData) {
        const [action, requestId] = callbackData.split(':');
        try {
            if (action === 'approve') {
                const response = await this.httpClient.post(`/approve/${requestId}`, {});
                if (response.success) {
                    return `✅ Request ${requestId.slice(0, 8)} approved and executed.\n\nResult: ${response.result || 'Command completed'}`;
                }
                else {
                    return `❌ Failed to approve request ${requestId.slice(0, 8)}`;
                }
            }
            else if (action === 'deny') {
                const response = await this.httpClient.post(`/deny/${requestId}`, {});
                if (response.success) {
                    return `🚫 Request ${requestId.slice(0, 8)} denied.`;
                }
                else {
                    return `❌ Failed to deny request ${requestId.slice(0, 8)}`;
                }
            }
            else {
                return '❓ Unknown action';
            }
        }
        catch (error) {
            this.logger.error('Failed to handle inline callback', {
                callbackData,
                error: error.message,
            });
            return `❌ Error: ${error.message}`;
        }
    }
    // ==========================================================================
    // Telegram Webhook Handling
    // ==========================================================================
    async handleTelegramMessage(message) {
        const text = message.text;
        const chatId = message.chat.id;
        // Check if it's a command
        if (text?.startsWith('/kc ')) {
            const parts = text.split(' ');
            const subCommand = parts[1];
            const args = parts.slice(2);
            switch (subCommand) {
                case 'request':
                    // Usage: /kc request <command> [--reason <reason>]
                    const reasonIndex = args.indexOf('--reason');
                    const command = reasonIndex !== -1 ? args.slice(0, reasonIndex).join(' ') : args.join(' ');
                    const reason = reasonIndex !== -1 ? args.slice(reasonIndex + 1).join(' ') : '';
                    if (!command) {
                        this.sendMessage(chatId, 'Usage: /kc request <command> [--reason <reason>]');
                        return;
                    }
                    await this.handleRequestCommand(chatId, command, reason);
                    break;
                case 'list':
                    await this.handleListCommand(chatId);
                    break;
                default:
                    this.sendMessage(chatId, `Unknown KingCrab command: ${subCommand}\n\nAvailable:\n/kc request <cmd> [--reason <reason>]\n/kc list`);
            }
        }
    }
    async handleRequestCommand(chatId, command, reason) {
        try {
            const response = await this.httpClient.post('/request', {
                command,
                reason: reason || 'Requested via Telegram',
            });
            if (!response.success || !response.request) {
                this.sendMessage(chatId, `❌ Failed to create request: ${response.error}`);
                return;
            }
            const req = response.request;
            const shortId = req.id.slice(0, 8);
            const message = `
🔐 *KingCrab Request ${shortId}*

Command: \`${req.command}\`
Reason: ${req.reason}
Status: ${req.status}
Time: ${new Date(req.timestamp).toLocaleString()}

Use buttons below to approve or deny this request.
`;
            await this.sendMessageWithButtons(chatId, message, [
                [
                    { text: '✅ Approve', callback_data: `approve:${req.id}` },
                    { text: '🚫 Deny', callback_data: `deny:${req.id}` },
                ],
            ]);
        }
        catch (error) {
            this.logger.error('Failed to handle /kc request', { error: error.message });
            this.sendMessage(chatId, `❌ Error creating request: ${error.message}`);
        }
    }
    async handleListCommand(chatId) {
        try {
            const response = await this.httpClient.get('/requests');
            if (!response.requests || response.requests.length === 0) {
                this.sendMessage(chatId, '📭 No requests found.');
                return;
            }
            const pending = response.requests.filter(r => r.status === 'pending');
            const message = `
📋 *KingCrab Requests*

Total: ${response.requests.length}
Pending: ${pending.length}

${pending
                .map(r => `• ${r.id.slice(0, 8)}: \`${r.command}\`\n  Reason: ${r.reason}\n  ${new Date(r.timestamp).toLocaleString()}`)
                .join('\n\n') || 'No pending requests'}
      `;
            this.sendMessage(chatId, message);
        }
        catch (error) {
            this.logger.error('Failed to handle /kc list', { error: error.message });
            this.sendMessage(chatId, `❌ Error listing requests: ${error.message}`);
        }
    }
    // Helper methods for Telegram
    async sendMessage(chatId, text, options = {}) {
        // This will be called by the framework's Telegram integration
        // The framework provides this method
        if (typeof this.sendTelegramMessage === 'function') {
            await this.sendTelegramMessage(chatId, text, options);
        }
    }
    async sendMessageWithButtons(chatId, text, inlineKeyboard) {
        if (typeof this.sendTelegramMessageWithButtons === 'function') {
            await this.sendTelegramMessageWithButtons(chatId, text, inlineKeyboard);
        }
    }
    // ==========================================================================
    // UI Assets
    // ==========================================================================
    getUIPath() {
        const __filename = fileURLToPath(import.meta.url);
        const __dirname = dirname(__filename);
        return resolve(__dirname, 'ui.html');
    }
}
// ============================================================================
// Export
// ============================================================================
export default KingCrabPlugin;
