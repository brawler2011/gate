import * as operations from '@/contracts/core/v1/api';
import {HttpError} from '@/lib/fetch';
import {notFound, redirect} from '@/lib/routeErrors';

export type ApiError = {status: number; message: string; requestId?: string};
type ApiResult<R> = [error: ApiError, data: null] | [error: null, data: R];
export type ApiFacade = {
  [K in keyof typeof operations as typeof operations[K] extends (...args: never[]) => Promise<unknown>
    ? K : never]: typeof operations[K] extends (...args: infer A) => Promise<infer R>
    ? (...args: A) => Promise<ApiResult<R>> : never;
};

export const api = new Proxy(operations, {
  get: (target, operation: keyof typeof operations) => async (...args: unknown[]) => {
    try {
      const fn = target[operation] as (...args: unknown[]) => Promise<unknown>;
      return [null, await fn(...args)];
    } catch (error) {
      if (error instanceof HttpError) {
        const requestId = error.body.request_id;
        if (error.status !== 401) {
          console.error('API Error:', {operation, status: error.status,
            requestId: requestId && /^[A-Za-z0-9._:-]{1,128}$/.test(requestId) ? requestId : undefined,
            errorType: 'ApiError'});
        }
        return [{status: error.status, message: error.message, requestId}, null];
      }
      console.error('Unknown API Error:', {operation, status: 500,
        errorType: error instanceof Error ? 'Error' : 'Unknown'});
      return [{status: 500, message: 'Неизвестная ошибка'}, null];
    }
  },
}) as unknown as ApiFacade;

export const unwrap = <A extends unknown[], R>(fn: (...args: A) => Promise<ApiResult<R>>) =>
  async (...args: A): Promise<R> => {
    const [error, data] = await fn(...args);
    if (error) {
      if (error.status === 404) {
        notFound();
      }
      if (error.status === 401) {
        redirect('/auth/login');
      }
      throw error;
    }
    return data;
  };
