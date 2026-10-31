export class HttpError extends Error {
  constructor(
    public status: number,
    public body: { message?: string; request_id?: string },
    public statusText: string,
  ) {
    super(body.message ?? statusText);
  }
}

export const apiFetch = async <T>(
  url: string,
  options: RequestInit = {},
): Promise<T> => {
  const response = await fetch(`/api${url}`, {
    ...options,
    credentials: "include",
  });
  if (!response.ok) {
    const decoded: unknown = await response.json().catch(() => null);
    const body = decoded && typeof decoded === "object" ? decoded : {};
    throw new HttpError(response.status, body, response.statusText);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const contentType = response.headers.get("content-type") ?? "";
  if (/\/tests\/[^?]+/.test(url)) {
    return response.blob() as Promise<T>;
  }
  if (contentType.startsWith("text/")) {
    return response.text() as Promise<T>;
  }
  if (contentType.includes("json")) {
    return response.json() as Promise<T>;
  }
  return response.blob() as Promise<T>;
};
