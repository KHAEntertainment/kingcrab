/**
 * OpenClaw Plugin Core Stub
 *
 * This is a temporary local stub for the @openclaw/plugin-core package.
 * Once @khentertainment/plugin-core is published to npm, this stub should be
 * deleted and the import changed back to '@khentertainment/plugin-core'.
 */
export type PluginConfig = Record<string, any>;
export interface Logger {
    info(msg: string, data?: any): void;
    error(msg: string, data?: any): void;
    warn(msg: string, data?: any): void;
    debug(msg: string, data?: any): void;
}
export declare class Plugin {
    logger: Logger;
    constructor(options: {
        name: string;
        version: string;
        description: string;
        author: string;
    });
    onLoad(config: PluginConfig): Promise<void>;
    onUnload(): Promise<void>;
    getRoutes(): Array<{
        method: string;
        path: string;
        handler: Function;
    }>;
    getUIPath(): string;
}
export declare class HTTPClient {
    private baseURL;
    private timeout;
    constructor(options: {
        baseURL: string;
        timeout: number;
    });
    get<T>(path: string): Promise<T>;
    post<T>(path: string, body: any): Promise<T>;
}
