import {notFound, redirect} from "next/navigation";
import {cache} from "react";

import type {ApiResult} from "./api";

export const unwrap = <A extends unknown[], R>(
  fn: (...args: A) => Promise<ApiResult<R>>
): ((...args: A) => Promise<R>) => {
  return async (...args: A): Promise<R> => {
    const [error, data] = await fn(...args);
    if (error) {
      if (error.status === 404) {
        notFound();
      }
      if (error.status === 401) {
        redirect("/auth/login");
      }
      throw error;
    }
    return data;
  };
};

export const unwrapAndCache = cache(unwrap);
