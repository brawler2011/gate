import {registerOTel} from "@vercel/otel";

import {env} from "@/lib/env";

/**
 * Next.js instrumentation hook.
 * Loaded once per Node.js process before the app starts.
 * @see https://nextjs.org/docs/app/api-reference/file-conventions/instrumentation
 */
export const register = async (): Promise<void> => {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    registerOTel({
      serviceName: env.getServerOtelServiceName(),
    });
  }
};
