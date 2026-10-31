import {readdirSync} from "node:fs";

import {defineConfig} from "orval";

export default defineConfig(
  Object.fromEntries(
    readdirSync("contracts", {recursive: true})
      .map(String)
      .filter((path) => path.endsWith("openapi.yaml"))
      .map((path) => `contracts/${path}`)
      .sort()
      .map((input) => {
        const name = input
          .replace(/^contracts\//, "")
          .replace(/\/openapi.yaml$/, "");
        return [
          name,
          {
            input,
            output: {
              baseUrl: "/api",
              target: `frontend/contracts/${name}/api.ts`,
              schemas: `frontend/contracts/${name}/models`,
              client: "react-query",
              httpClient: "fetch",
              clean: true,
              urlEncodeParameters: true,
              override: {
                requestOptions: {credentials: "include"},
                query: {version: 5},
                fetch: {includeHttpResponseReturnType: false, forceSuccessResponse: true},
              },
            },
          },
        ];
      }),
  ),
);
