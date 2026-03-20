/**
 * OpenClaw Plugin Core Stub
 *
 * This is a temporary local stub for the @openclaw/plugin-core package.
 * Once @khentertainment/plugin-core is published to npm, this stub should be
 * deleted and the import changed back to '@khentertainment/plugin-core'.
 */

// PluginConfig - just an object for now
export type PluginConfig = Record<string, any>;

// Logger interface
export interface Logger {
  info(msg: string, data?: any): void;
  error(msg: string, data?: any): void;
  warn(msg: string, data?: any): void;
  debug(msg: string, data?: any): void;
}

// Plugin base class
export class Plugin {
  logger: Logger;

  constructor(options: { name: string; version: string; description: string; author: string }) {
    this.logger = {
      info: (msg: string, data?: any) => console.log(`[${options.name}] INFO:`, msg, data || ''),
      error: (msg: string, data?: any) => console.error(`[${options.name}] ERROR:`, msg, data || ''),
      warn: (msg: string, data?: any) => console.warn(`[${options.name}] WARN:`, msg, data || ''),
      debug: (msg: string, data?: any) => console.debug(`[${options.name}] DEBUG:`, msg, data || ''),
    };
  }

  async onLoad(config: PluginConfig): Promise<void> {}
  async onUnload(): Promise<void> {}

  getRoutes(): Array<{
    method: string;
    path: string;
    handler: Function;
  }> {
    return [];
  }

  getUIPath(): string {
    return '';
  }
}

// HTTPClient - used for daemon communication
export class HTTPClient {
  private baseURL: string;
  private timeout: number;

  constructor(options: { baseURL: string; timeout: number }) {
    this.baseURL = options.baseURL;
    this.timeout = options.timeout;
  }

  async get<T>(path: string): Promise<T> {
    const url = `${this.baseURL}${path}`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        method: 'GET',
        signal: controller.signal,
      });

      if (!response.ok) {
        const bodyText = await response.text();
        throw new Error(`HTTP ${response.status}: ${response.statusText}${bodyText ? ` - ${bodyText}` : ''}`);
      }

      return await response.json() as T;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async post<T>(path: string, body: any): Promise<T> {
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
        const bodyText = await response.text();
        throw new Error(`HTTP ${response.status}: ${response.statusText}${bodyText ? ` - ${bodyText}` : ''}`);
      }

      return await response.json() as T;
    } finally {
      clearTimeout(timeoutId);
    }
  }
}