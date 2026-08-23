import {GlobalWindow} from "happy-dom";

if (!process.env.BACKEND_API_URL) {
  process.env.BACKEND_API_URL = "http://localhost:8080";
}

const dummyDoc = {
  getElementsByTagName: () => [],
  head: null,
  body: null,
  createElement: () => ({}),
  addEventListener: () => {},
  removeEventListener: () => {},
};

const dummyLocation = {
  href: "http://localhost/",
  origin: "http://localhost",
  protocol: "http:",
  host: "localhost",
  hostname: "localhost",
  port: "",
  pathname: "/",
  search: "",
  hash: "",
};

const dummyNavigator = {
  userAgent: "Node",
};

export const setupDOMEnvironment = (): (() => void) => {
  const win = new GlobalWindow();

  // matchMedia mock
  Object.defineProperty(win, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });

  const definedKeys: string[] = [];

  // Register window properties
  const winProps = Object.getOwnPropertyNames(win);
  for (const prop of winProps) {
    if (prop === "undefined" || prop === "NaN" || prop === "Infinity" || prop.startsWith("_")) {
      continue;
    }
    if (!(prop in globalThis) || prop === "window" || prop === "document" || prop === "navigator" || prop === "location") {
      try {
        const val = (win as unknown as Record<string, unknown>)[prop];
        Object.defineProperty(globalThis, prop, {
          value: typeof val === "function" && !val.prototype ? (val as (...args: unknown[]) => unknown).bind(win) : val,
          configurable: true,
          writable: true,
        });
        definedKeys.push(prop);
      } catch {
        // Skip unassignable properties
      }
    }
  }

  return () => {
    for (const key of definedKeys) {
      try {
        if (key === "document") {
          Object.defineProperty(globalThis, "document", {
            value: dummyDoc,
            configurable: true,
            writable: true,
          });
        } else if (key === "location") {
          Object.defineProperty(globalThis, "location", {
            value: dummyLocation,
            configurable: true,
            writable: true,
          });
        } else if (key === "navigator") {
          Object.defineProperty(globalThis, "navigator", {
            value: dummyNavigator,
            configurable: true,
            writable: true,
          });
        } else if (key === "window") {
          delete (globalThis as Record<string, unknown>).window;
        } else {
          delete (globalThis as Record<string, unknown>)[key];
        }
      } catch {
        // Skip
      }
    }
  };
};
