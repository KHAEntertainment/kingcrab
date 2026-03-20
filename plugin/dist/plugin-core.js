/**
 * OpenClaw Plugin Core Stub
 *
 * This is a temporary local stub for the @openclaw/plugin-core package.
 * Once @khentertainment/plugin-core is published to npm, this stub should be
 * deleted and the import changed back to '@khentertainment/plugin-core'.
 */
// Plugin base class
export class Plugin {
    logger;
    constructor(options) {
        this.logger = {
            info: (msg, data) => console.log(`[${options.name}] INFO:`, msg, data || ''),
            error: (msg, data) => console.error(`[${options.name}] ERROR:`, msg, data || ''),
            warn: (msg, data) => console.warn(`[${options.name}] WARN:`, msg, data || ''),
            debug: (msg, data) => console.debug(`[${options.name}] DEBUG:`, msg, data || ''),
        };
    }
    async onLoad(config) { }
    async onUnload() { }
    getRoutes() {
        return [];
    }
    getUIPath() {
        return '';
    }
}
// HTTPClient - used for daemon communication
export class HTTPClient {
    baseURL;
    timeout;
    constructor(options) {
        this.baseURL = options.baseURL;
        this.timeout = options.timeout;
    }
    async get(path) {
        const url = `${this.baseURL}${path}`;
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), this.timeout);
        try {
            const response = await fetch(url, {
                method: 'GET',
                signal: controller.signal,
            });
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return await response.json();
        }
        finally {
            clearTimeout(timeoutId);
        }
    }
    async post(path, body) {
        const url = `${this.baseURL}${path}`;
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), this.timeout);
        try {
            const response = await fetch(url, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(body),
                signal: controller.signal,
            });
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return await response.json();
        }
        finally {
            clearTimeout(timeoutId);
        }
    }
}
