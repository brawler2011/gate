import {CancelablePromise} from "@/contracts/core/v1/core/CancelablePromise";
import {FetchHttpRequest} from "@/contracts/core/v1/core/FetchHttpRequest";
import {catchErrorCodes, getHeaders, getResponseBody, sendRequest} from "@/contracts/core/v1/core/request";

import type {ApiRequestOptions} from "@/contracts/core/v1/core/ApiRequestOptions";

// The generated fetch transport decodes non-JSON bodies as text. Test assets
// require Blob responses to preserve bytes for preview, editing and downloads.
export class TestFilesHttpRequest extends FetchHttpRequest {
  public override request<T>(options: ApiRequestOptions): CancelablePromise<T> {
    if (options.method !== "GET" || options.url !== "/problems/{problemId}/tests/{name}") {
      return super.request<T>(options);
    }
    return new CancelablePromise(async (resolve, reject, onCancel) => {
      try {
        const problemId = encodeURIComponent(String(options.path?.problemId));
        const name = encodeURIComponent(String(options.path?.name));
        const url = `${this.config.BASE}/problems/${problemId}/tests/${name}`;
        const headers = await getHeaders(this.config, options);
        if (!onCancel.isCancelled) {
          const response = await sendRequest(this.config, options, url, undefined, undefined, headers, onCancel);
          const body = response.ok ? await response.blob() : await getResponseBody(response);
          catchErrorCodes(options, {
            url, ok: response.ok, status: response.status, statusText: response.statusText, body,
          });
          resolve(body as T);
        }
      } catch (error) {
        reject(error);
      }
    });
  }
}
