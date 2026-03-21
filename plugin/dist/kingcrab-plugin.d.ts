#!/usr/bin/env -S node
/**
 * KingCrab Plugin for OpenClaw
 * PAM for chat-based sudo approval workflows
 * Version: 0.1.0
 */
import { Plugin, PluginConfig } from './plugin-core.js';
declare class KingCrabPlugin extends Plugin {
    private config;
    private httpClient;
    private baseUrl;
    constructor();
    onLoad(config: PluginConfig): Promise<void>;
    onUnload(): Promise<void>;
    getRoutes(): {
        method: string;
        path: string;
        handler: (req: any, res: any) => Promise<void>;
    }[];
    private createRequest;
    private listRequests;
    private approveRequest;
    private denyRequest;
    handleInlineCallback(callbackData: string): Promise<string>;
    handleTelegramMessage(message: any): Promise<void>;
    private handleRequestCommand;
    private handleListCommand;
    private sendMessage;
    private sendMessageWithButtons;
    getUIPath(): string;
}
export default KingCrabPlugin;
