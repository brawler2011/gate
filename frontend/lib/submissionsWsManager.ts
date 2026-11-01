"use client";

type ListenerParams = {
  key: string;
  url: () => string;
  enabled: boolean;
  onMessage: (event: MessageEvent) => void;
  onConnectionError: (hasError: boolean) => void;
  onFatalConnectionError: (isFatal: boolean) => void;
  onStatusChange: (status: WsManagerStatus) => void;
  onResyncRequired: () => Promise<void>;
};

type Listener = ListenerParams & {id: number};
type Connection = {key: string; url: string};

export type WsManagerStatus =
  | {phase: 'idle'}
  | {phase: 'connecting'}
  | {phase: 'connected'}
  | {phase: 'reconnecting'; attempt: number; maxAttempts: number}
  | {phase: 'fatal'; attempt: number; maxAttempts: number};

const RECONNECT_DELAYS = [2000, 4000, 8000];
const CONNECTION_TIMEOUT = 10000;

const sameStream = (first: string, second: string): boolean => {
  try {
    const a = new URL(first);
    const b = new URL(second);
    a.searchParams.delete('since');
    b.searchParams.delete('since');
    return a.toString() === b.toString();
  } catch {
    return first === second;
  }
};

class SubmissionsWsManager {
  private listeners = new Map<number, Listener>();
  private nextId = 1;
  private ws: WebSocket | null = null;
  private active: Connection | null = null;
  private reconnectAttempt = 0;
  private reconnectTimeout: ReturnType<typeof setTimeout> | undefined;
  private connectionTimeout: ReturnType<typeof setTimeout> | undefined;
  private generation = 0;
  private resyncing = false;
  private status: WsManagerStatus = {phase: 'idle'};
  private statusKey: string | null = null;

  addListener(params: ListenerParams): number {
    const id = this.nextId++;
    this.listeners.set(id, {id, ...params});
    this.syncStatus(params);
    this.reconcile();
    return id;
  }

  updateListener(id: number, params: ListenerParams): void {
    if (!this.listeners.has(id)) {
      return;
    }
    this.listeners.set(id, {id, ...params});
    this.syncStatus(params);
    this.reconcile();
  }

  removeListener(id: number): void {
    this.listeners.delete(id);
    this.reconcile();
  }

  private forKey(key = this.active?.key): Listener[] {
    return [...this.listeners.values()].filter(listener => listener.enabled && listener.key === key);
  }

  private desired(): Connection | null {
    let latest: Listener | undefined;
    for (const listener of this.listeners.values()) {
      if (listener.enabled && (!latest || listener.id > latest.id)) {
        latest = listener;
      }
    }
    return latest ? {key: latest.key, url: latest.url()} : null;
  }

  private syncStatus(listener: ListenerParams) {
    if (listener.enabled) {
      listener.onStatusChange(listener.key === this.statusKey ? this.status : {phase: 'idle'});
    }
  }

  private notifyStatus(key: string | null, status: WsManagerStatus) {
    this.status = status;
    this.statusKey = key;
    this.forKey(key ?? undefined).forEach(listener => listener.onStatusChange(status));
  }

  private async resync(key: string): Promise<boolean> {
    const listeners = this.forKey(key);
    if (!listeners.length) {
      return false;
    }
    try {
      await Promise.all(listeners.map(listener => listener.onResyncRequired()));
      return true;
    } catch {
      this.forKey(key).forEach(listener => listener.onConnectionError(true));
      return false;
    }
  }

  private clearTimers() {
    clearTimeout(this.reconnectTimeout);
    clearTimeout(this.connectionTimeout);
    this.reconnectTimeout = undefined;
    this.connectionTimeout = undefined;
  }

  private closeSocket() {
    this.generation++;
    this.clearTimers();
    const socket = this.ws;
    this.ws = null;
    socket?.close();
    this.resyncing = false;
    this.reconnectAttempt = 0;
    this.forKey().forEach(listener => listener.onFatalConnectionError(false));
    this.notifyStatus(this.active?.key ?? null, {phase: 'idle'});
  }

  private connect(desired: Connection) {
    this.clearTimers();
    this.active = desired;
    if (this.status.phase !== 'reconnecting') {
      this.notifyStatus(desired.key, {phase: 'connecting'});
    }
    try {
      const socket = new WebSocket(desired.url);
      this.ws = socket;
      this.connectionTimeout = setTimeout(() => {
        if (this.ws === socket && socket.readyState !== WebSocket.OPEN) {
          socket.close();
        }
      }, CONNECTION_TIMEOUT);
      socket.onopen = () => {
        if (this.ws !== socket) {
          return;
        }
        clearTimeout(this.connectionTimeout);
        this.reconnectAttempt = 0;
        this.forKey().forEach(listener => {
          listener.onFatalConnectionError(false);
          listener.onConnectionError(false);
        });
        this.notifyStatus(this.active?.key ?? desired.key, {phase: 'connected'});
      };
      socket.onmessage = event => {
        if (this.ws === socket) {
          this.forKey().forEach(listener => listener.onMessage(event));
        }
      };
      socket.onclose = () => {
        if (this.ws !== socket) {
          return;
        }
        this.ws = null;
        clearTimeout(this.connectionTimeout);
        this.reconnect(true);
      };
    } catch {
      this.forKey().forEach(listener => listener.onConnectionError(true));
      this.reconnect(false);
    }
  }

  private reconnect(resync: boolean) {
    const desired = this.desired();
    if (!desired) {
      return;
    }
    if (this.reconnectAttempt >= RECONNECT_DELAYS.length) {
      this.forKey().forEach(listener => {
        listener.onConnectionError(true);
        listener.onFatalConnectionError(true);
      });
      this.notifyStatus(desired.key, {
        phase: 'fatal', attempt: this.reconnectAttempt, maxAttempts: RECONNECT_DELAYS.length,
      });
      return;
    }
    clearTimeout(this.reconnectTimeout);
    const delay = RECONNECT_DELAYS[this.reconnectAttempt++];
    this.notifyStatus(desired.key, {
      phase: 'reconnecting', attempt: this.reconnectAttempt, maxAttempts: RECONNECT_DELAYS.length,
    });
    this.reconnectTimeout = setTimeout(async () => {
      const current = this.desired();
      if (!current) {
        return;
      }
      const generation = this.generation;
      this.resyncing = resync;
      const ok = !resync || await this.resync(current.key);
      if (generation !== this.generation) {
        return;
      }
      this.resyncing = false;
      if (!ok) {
        this.reconnect(true);
        return;
      }
      const refreshed = this.desired();
      if (refreshed) {
        this.connect(refreshed);
      }
    }, delay);
  }

  private async switchConnection(desired: Connection) {
    this.closeSocket();
    this.active = desired;
    const generation = this.generation;
    this.resyncing = true;
    const ok = await this.resync(desired.key);
    if (generation !== this.generation) {
      return;
    }
    this.resyncing = false;
    const refreshed = this.desired();
    if (ok && refreshed) {
      this.connect(refreshed);
    }
  }

  private reconcile() {
    const desired = this.desired();
    if (!desired) {
      this.closeSocket();
      this.active = null;
      return;
    }
    const alive = this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING);
    if (alive && this.active && sameStream(this.active.url, desired.url)) {
      this.active = desired;
      this.notifyStatus(desired.key, this.status);
      return;
    }
    if (this.active?.key !== desired.key || this.active.url !== desired.url) {
      void this.switchConnection(desired);
    } else if (!alive && !this.resyncing && this.reconnectTimeout === undefined) {
      this.reconnect(false);
    }
  }
}

export const submissionsWsManager = new SubmissionsWsManager();
