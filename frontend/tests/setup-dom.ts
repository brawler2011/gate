import {notifyManager} from '@tanstack/react-query';
import {GlobalWindow} from 'happy-dom';

notifyManager.setScheduler(queueMicrotask);

// Keep a DOM installed between suites so React's queued scheduler work can finish.
const win = new GlobalWindow();
Object.defineProperty(win, 'matchMedia', {
  value: (query: string) => ({matches: false, media: query, onchange: null,
    addListener: () => {}, removeListener: () => {}, addEventListener: () => {},
    removeEventListener: () => {}, dispatchEvent: () => false}),
});

for (const prop of Object.getOwnPropertyNames(win)) {
  if (['undefined', 'NaN', 'Infinity'].includes(prop) || prop.startsWith('_')) {
    continue;
  }
  if (!(prop in globalThis) || ['window', 'document', 'navigator', 'location'].includes(prop)) {
    try {
      const value = (win as unknown as Record<string, unknown>)[prop];
      Object.defineProperty(globalThis, prop, {
        value: typeof value === 'function' && !value.prototype ? value.bind(win) : value,
        configurable: true, writable: true,
      });
    } catch {
      // Some runtime globals cannot be reassigned.
    }
  }
}
