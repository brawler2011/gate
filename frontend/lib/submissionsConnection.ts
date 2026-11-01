type ConnectionOptions = {
  url: () => string;
  onMessage: (event: MessageEvent) => void;
  onStatusChange: (status: SubmissionsConnectionStatus) => void;
  onResyncRequired?: () => Promise<void>;
};

export type SubmissionsConnectionStatus =
  | {phase: 'idle'}
  | {phase: 'connecting'}
  | {phase: 'connected'}
  | {phase: 'reconnecting'; attempt: number; maxAttempts: number}
  | {phase: 'fatal'; attempt: number; maxAttempts: number};

const RECONNECT_DELAYS = [2000, 4000, 8000];
const CONNECTION_TIMEOUT = 10000;

export const connectSubmissions = (options: ConnectionOptions): (() => void) => {
  let disposed = false;
  let socket: WebSocket | null = null;
  let attempt = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const retry = (resync: boolean) => {
    if (disposed) {
      return;
    }
    if (attempt === RECONNECT_DELAYS.length) {
      options.onStatusChange({phase: 'fatal', attempt, maxAttempts: RECONNECT_DELAYS.length});
      return;
    }
    const delay = RECONNECT_DELAYS[attempt++];
    options.onStatusChange({phase: 'reconnecting', attempt, maxAttempts: RECONNECT_DELAYS.length});
    timer = setTimeout(() => void connect(resync), delay);
  };

  const connect = async (resync: boolean) => {
    if (resync) {
      try {
        await options.onResyncRequired?.();
      } catch {
        retry(true);
        return;
      }
    }
    if (disposed) {
      return;
    }
    try {
      const ws = new WebSocket(options.url());
      socket = ws;
      timer = setTimeout(() => ws.close(), CONNECTION_TIMEOUT);
      ws.onopen = () => {
        if (disposed || socket !== ws) {
          return;
        }
        clearTimeout(timer);
        attempt = 0;
        options.onStatusChange({phase: 'connected'});
      };
      ws.onmessage = event => {
        if (!disposed && socket === ws) {
          options.onMessage(event);
        }
      };
      ws.onclose = () => {
        if (!disposed && socket === ws) {
          socket = null;
          clearTimeout(timer);
          retry(true);
        }
      };
    } catch {
      retry(false);
    }
  };

  options.onStatusChange({phase: 'connecting'});
  void connect(true);
  return () => {
    disposed = true;
    clearTimeout(timer);
    socket?.close();
  };
};
