'use client';

import { SubmissionsWsCloseCode } from '@contracts/observer/v1';

type ListenerParams = {
  key: string;
  url: string;
  enabled: boolean;
  onMessage: (event: MessageEvent) => void;
  onConnectionError: (hasError: boolean) => void;
  onFatalConnectionError: (isFatal: boolean) => void;
  onStatusChange: (status: WsManagerStatus) => void;
  onResyncRequired: () => Promise<void>;
};

type Listener = ListenerParams & { id: number };

export type WsManagerStatus =
  | { phase: 'idle' }
  | { phase: 'connecting' }
  | { phase: 'connected' }
  | { phase: 'reconnecting'; attempt: number; maxAttempts: number }
  | { phase: 'fatal'; attempt: number; maxAttempts: number };

const RECONNECT_DELAYS = [2000, 4000, 8000];
const MAX_RECONNECT_ATTEMPTS = 3;
const CONNECTION_TIMEOUT = 10000;
const RESYNC_CLOSE_CODES = new Set<number>([
  SubmissionsWsCloseCode.history_lost,
  SubmissionsWsCloseCode.invalid_range,
]);

const isDev = process.env.NODE_ENV === 'development';

function log(...args: unknown[]) {
  if (isDev) {
    console.log('[WSM]', ...args);
  }
}

type SocketState = 'idle' | 'connecting' | 'open' | 'closing';

class SubmissionsWsManager {
  private listeners = new Map<number, Listener>();
  private nextListenerId = 1;

  private ws: WebSocket | null = null;
  private activeKey: string | null = null;
  private activeUrl: string | null = null;
  private state: SocketState = 'idle';

  private reconnectAttempt = 0;
  private reconnectTimeout: NodeJS.Timeout | null = null;
  private connectionTimeout: NodeJS.Timeout | null = null;
  private connectToken = 0;
  private manualClose = false;
  private isResyncing = false;
  private reconnectCount = 0;
  private resyncCount = 0;
  private inReconnectCycle = false;
  private currentStatus: WsManagerStatus = { phase: 'idle' };
  private statusKey: string | null = null;

  addListener(params: ListenerParams): number {
    const id = this.nextListenerId++;
    this.listeners.set(id, { id, ...params });
    this.syncListenerStatus(id);
    this.reconcile('addListener');
    return id;
  }

  updateListener(id: number, params: ListenerParams): void {
    const existing = this.listeners.get(id);
    if (!existing) {
      return;
    }
    this.listeners.set(id, { id, ...params });
    this.syncListenerStatus(id);
    this.reconcile('updateListener');
  }

  removeListener(id: number): void {
    this.listeners.delete(id);
    this.reconcile('removeListener');
  }

  private clearTimers() {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }
    if (this.connectionTimeout) {
      clearTimeout(this.connectionTimeout);
      this.connectionTimeout = null;
    }
  }

  private isSameStreamUrl(a: string | null, b: string): boolean {
    if (!a) {
      return false;
    }
    try {
      const ua = new URL(a);
      const ub = new URL(b);
      if (ua.origin !== ub.origin || ua.pathname !== ub.pathname) {
        return false;
      }
      const pa = new URLSearchParams(ua.search);
      const pb = new URLSearchParams(ub.search);
      pa.delete('since');
      pb.delete('since');
      return pa.toString() === pb.toString();
    } catch {
      return a === b;
    }
  }

  private canRetargetWithoutReconnect(desired: { key: string; url: string }): boolean {
    const wsAlive = Boolean(this.ws) && (this.ws!.readyState === WebSocket.OPEN || this.ws!.readyState === WebSocket.CONNECTING);
    return wsAlive && this.isSameStreamUrl(this.activeUrl, desired.url);
  }

  private retargetActiveConnection(desired: { key: string; url: string }) {
    this.activeKey = desired.key;
    this.activeUrl = desired.url;
    this.notifyStatusForKey(desired.key, this.currentStatus);
  }

  private getDesiredConnection(): { key: string; url: string } | null {
    let selected: Listener | null = null;
    for (const listener of this.listeners.values()) {
      if (!listener.enabled) continue;
      if (!selected || listener.id > selected.id) {
        selected = listener;
      }
    }

    if (!selected) {
      return null;
    }

    return { key: selected.key, url: selected.url };
  }

  private getActiveListeners(): Listener[] {
    const key = this.activeKey;
    if (!key) {
      return [];
    }
    return [...this.listeners.values()].filter((listener) => listener.enabled && listener.key === key);
  }

  private getListenersForKey(key: string): Listener[] {
    return [...this.listeners.values()].filter((listener) => listener.enabled && listener.key === key);
  }

  private syncListenerStatus(id: number) {
    const listener = this.listeners.get(id);
    if (!listener || !listener.enabled) {
      return;
    }
    if (this.statusKey && listener.key === this.statusKey) {
      listener.onStatusChange(this.currentStatus);
      return;
    }
    listener.onStatusChange({ phase: 'idle' });
  }

  private notifyConnectionError(hasError: boolean) {
    for (const listener of this.getActiveListeners()) {
      listener.onConnectionError(hasError);
    }
  }

  private notifyFatalConnectionError(isFatal: boolean) {
    for (const listener of this.getActiveListeners()) {
      listener.onFatalConnectionError(isFatal);
    }
  }

  private notifyStatusForKey(key: string | null, status: WsManagerStatus) {
    this.currentStatus = status;
    this.statusKey = key;
    if (!key) {
      return;
    }
    for (const listener of this.getListenersForKey(key)) {
      listener.onStatusChange(status);
    }
  }

  private routeMessage(event: MessageEvent) {
    for (const listener of this.getActiveListeners()) {
      listener.onMessage(event);
    }
  }

  private async resyncForKey(key: string): Promise<boolean> {
    if (this.isResyncing) {
      return false;
    }

    const targets = [...this.listeners.values()].filter((listener) => listener.enabled && listener.key === key);
    if (targets.length === 0) {
      return false;
    }

    this.isResyncing = true;
    try {
      await Promise.all(targets.map((listener) => listener.onResyncRequired()));
      return true;
    } catch (error) {
      log('Resync failed', { key, error });
      this.notifyConnectionError(true);
      return false;
    } finally {
      this.isResyncing = false;
    }
  }

  private closeSocket(reason: string) {
    const key = this.activeKey;
    this.clearTimers();
    if (this.ws) {
      this.manualClose = true;
      this.state = 'closing';
      this.ws.close();
      this.ws = null;
    }
    this.state = 'idle';
    this.reconnectAttempt = 0;
    this.inReconnectCycle = false;
    this.notifyFatalConnectionError(false);
    this.notifyStatusForKey(key, { phase: 'idle' });
  }

  private connect(url: string, key: string, reason: string) {
    this.clearTimers();

    if (this.state === 'connecting' || this.state === 'open') {
      log('Skip connect: state busy', { state: this.state, reason });
      return;
    }

    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      log('Skip connect: socket already alive', { reason });
      return;
    }

    const token = ++this.connectToken;
    this.manualClose = false;
    this.state = 'connecting';
    if (!this.inReconnectCycle) {
      this.notifyStatusForKey(key, { phase: 'connecting' });
    }

    log('Connecting', { token, reason, url });

    try {
      const ws = new WebSocket(url);
      this.ws = ws;

      this.connectionTimeout = setTimeout(() => {
        if (token !== this.connectToken) return;
        if (ws.readyState !== WebSocket.OPEN) {
          log('Connection timeout', { token });
          ws.close();
        }
      }, CONNECTION_TIMEOUT);

      ws.onopen = () => {
        if (token !== this.connectToken) return;
        this.state = 'open';
        if (this.connectionTimeout) {
          clearTimeout(this.connectionTimeout);
          this.connectionTimeout = null;
        }
        this.reconnectAttempt = 0;
        this.inReconnectCycle = false;
        this.notifyFatalConnectionError(false);
        this.notifyConnectionError(false);
        const openKey = this.activeKey ?? key;
        this.notifyStatusForKey(openKey, { phase: 'connected' });
        log('Connected', { token, key: openKey });
      };

      ws.onmessage = (event) => {
        if (token !== this.connectToken) return;
        this.routeMessage(event);
      };

      ws.onerror = (error) => {
        if (token !== this.connectToken) return;
        log('Socket error', { token, error });
      };

      ws.onclose = (event) => {
        if (token !== this.connectToken) return;

        this.state = 'idle';
        if (this.ws === ws) {
          this.ws = null;
        }
        if (this.connectionTimeout) {
          clearTimeout(this.connectionTimeout);
          this.connectionTimeout = null;
        }

        log('Disconnected', {
          token,
          code: event.code,
          reason: event.reason,
          manual: this.manualClose,
          key,
        });

        if (this.manualClose) {
          this.manualClose = false;
          return;
        }

        const desired = this.getDesiredConnection();
        if (!desired || !this.activeKey || !this.activeUrl || desired.key !== this.activeKey || desired.url !== this.activeUrl) {
          return;
        }

        if (RESYNC_CLOSE_CODES.has(event.code)) {
          void this.reconnectWithResync(`socket_close:${event.code}`);
          return;
        }

        if (event.code === 1000) {
          void this.reconnectWithoutResync('socket_close:1000');
          return;
        }

        void this.reconnectWithoutResync(`socket_close:${event.code}`);
      };
    } catch (error) {
      this.state = 'idle';
      log('Connect failed', { reason, error });
      this.notifyConnectionError(true);
      void this.reconnectWithoutResync('connect_failure');
    }
  }

  private async reconnectWithoutResync(reason: string) {
    const desired = this.getDesiredConnection();
    if (!desired) {
      return;
    }

    if (this.reconnectAttempt >= MAX_RECONNECT_ATTEMPTS) {
      log('Max reconnect attempts reached', { reason });
      this.notifyConnectionError(true);
      this.notifyFatalConnectionError(true);
      this.inReconnectCycle = false;
      this.notifyStatusForKey(desired.key, {
        phase: 'fatal',
        attempt: this.reconnectAttempt,
        maxAttempts: MAX_RECONNECT_ATTEMPTS,
      });
      return;
    }

    const delay = RECONNECT_DELAYS[this.reconnectAttempt] ?? RECONNECT_DELAYS[RECONNECT_DELAYS.length - 1];
    const attempt = this.reconnectAttempt + 1;
    this.reconnectAttempt++;
    this.reconnectCount++;
    this.inReconnectCycle = true;
    this.notifyStatusForKey(desired.key, { phase: 'reconnecting', attempt, maxAttempts: MAX_RECONNECT_ATTEMPTS });

    log('Schedule reconnect (no resync)', { reason, attempt, delay, reconnectCount: this.reconnectCount });
    this.reconnectTimeout = setTimeout(() => {
      const currentDesired = this.getDesiredConnection();
      if (!currentDesired) {
        return;
      }

      this.activeKey = currentDesired.key;
      this.activeUrl = currentDesired.url;
      this.connect(currentDesired.url, currentDesired.key, 'reconnect_no_resync');
    }, delay);
  }

  private async reconnectWithResync(reason: string) {
    const desired = this.getDesiredConnection();
    if (!desired) {
      return;
    }

    if (this.reconnectAttempt >= MAX_RECONNECT_ATTEMPTS) {
      log('Max reconnect attempts reached', { reason });
      this.notifyConnectionError(true);
      this.notifyFatalConnectionError(true);
      this.inReconnectCycle = false;
      this.notifyStatusForKey(desired.key, {
        phase: 'fatal',
        attempt: this.reconnectAttempt,
        maxAttempts: MAX_RECONNECT_ATTEMPTS,
      });
      return;
    }

    const delay = RECONNECT_DELAYS[this.reconnectAttempt] ?? RECONNECT_DELAYS[RECONNECT_DELAYS.length - 1];
    const attempt = this.reconnectAttempt + 1;
    this.reconnectAttempt++;
    this.reconnectCount++;
    this.inReconnectCycle = true;
    this.notifyStatusForKey(desired.key, { phase: 'reconnecting', attempt, maxAttempts: MAX_RECONNECT_ATTEMPTS });

    log('Schedule reconnect', { reason, attempt, delay });
    this.reconnectTimeout = setTimeout(async () => {
      const currentDesired = this.getDesiredConnection();
      if (!currentDesired) {
        return;
      }

      const ok = await this.resyncForKey(currentDesired.key);
      this.resyncCount++;
      log('Resync attempt done', { ok, reason, resyncCount: this.resyncCount });
      if (!ok) {
        return;
      }

      const finalDesired = this.getDesiredConnection();
      if (!finalDesired) {
        return;
      }

      this.activeKey = finalDesired.key;
      this.activeUrl = finalDesired.url;
      this.connect(finalDesired.url, finalDesired.key, 'reconnect_with_resync');
    }, delay);
  }

  private async switchConnection(desired: { key: string; url: string }, reason: string) {
    this.closeSocket(`switch:${reason}`);
    this.inReconnectCycle = false;

    const ok = await this.resyncForKey(desired.key);
    if (!ok) {
      return;
    }

    const refreshed = this.getDesiredConnection();
    if (!refreshed) {
      return;
    }

    this.activeKey = refreshed.key;
    this.activeUrl = refreshed.url;
    this.connect(refreshed.url, refreshed.key, `switch:${reason}`);
  }

  private reconcile(reason: string) {
    const desired = this.getDesiredConnection();

    if (!desired) {
      this.closeSocket(`reconcile:${reason}:no_listeners`);
      this.activeKey = null;
      this.activeUrl = null;
      return;
    }

    if (this.ws && this.ws.readyState === WebSocket.OPEN && this.activeKey === desired.key) {
      if (this.currentStatus.phase !== 'connected') {
        this.notifyStatusForKey(desired.key, { phase: 'connected' });
      }
      return;
    }

    const changed = this.activeKey !== desired.key || this.activeUrl !== desired.url;
    if (changed) {
      if (this.canRetargetWithoutReconnect(desired)) {
        this.retargetActiveConnection(desired);
        return;
      }
      void this.switchConnection(desired, reason);
      return;
    }

    if (!this.ws || this.ws.readyState === WebSocket.CLOSED || this.ws.readyState === WebSocket.CLOSING) {
      void this.reconnectWithoutResync(`reconcile:${reason}:socket_not_open`);
    }
  }
}

export const submissionsWsManager = new SubmissionsWsManager();
