export class TestWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: TestWebSocket[] = [];
  static failConnections = false;

  readyState = TestWebSocket.CONNECTING;
  closeCalls = 0;
  onopen: (() => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: {code: number; reason: string}) => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) {
    if (TestWebSocket.failConnections) {
      throw new Error("connection failed");
    }
    TestWebSocket.instances.push(this);
  }

  open(): void {
    this.readyState = TestWebSocket.OPEN;
    this.onopen?.();
  }

  message(data: unknown): void {
    this.onmessage?.({data: JSON.stringify(data)} as MessageEvent);
  }

  disconnect(code = 1006): void {
    this.readyState = TestWebSocket.CLOSED;
    this.onclose?.({code, reason: "closed"});
  }

  close(): void {
    this.closeCalls++;
    this.readyState = TestWebSocket.CLOSING;
    queueMicrotask(() => this.disconnect(1000));
  }
}

export const flushMicrotasks = async (): Promise<void> => {
  for (let i = 0; i < 10; i++) {
    await Promise.resolve();
  }
};
