import "./setup-dom";

import {afterAll, afterEach, beforeAll, describe, expect, it, mock} from "bun:test";
import React from "react";
import {renderToString} from "react-dom/server";

import {useEntityCollection, useEntityMutation} from "@/hooks/useEntityManagement";

import {act, cleanup, renderHook, setupDOMEnvironment, waitFor} from "./test-utils";

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return {promise, resolve};
};

describe("Shared entity management hooks", () => {
  let cleanupDOM: () => void;
  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });
  afterEach(cleanup);
  afterAll(() => {
    cleanupDOM();
  });

  it("does not fetch during server rendering and starts loading on the client", async () => {
    const data = deferred<string[]>();
    const load = mock(() => data.promise);
    const Probe = () => {
      const {items, loading} = useEntityCollection(load);
      return <span>{loading ? "loading" : items.join(",")}</span>;
    };
    expect(renderToString(<Probe />)).toContain("loading");
    expect(load).not.toHaveBeenCalled();
    const {result} = renderHook(() => useEntityCollection(load));
    expect(load).toHaveBeenCalledTimes(1);
    expect(result.current.loading).toBe(true);
    await act(async () => {
      data.resolve(["alice"]);
    });
    expect(result.current.items).toEqual(["alice"]);
    expect(result.current.loading).toBe(false);
  });

  it("retains rows on API errors and reloads after success", async () => {
    const load = mock<() => Promise<string[] | undefined>>(async () => ["alice"]);
    const {result} = renderHook(() => useEntityCollection(load));
    await waitFor(() => expect(result.current.items).toEqual(["alice"]));
    load.mockResolvedValueOnce(undefined);
    await act(async () => {
      await result.current.reload();
    });
    expect(result.current.items).toEqual(["alice"]);
    expect(result.current.loading).toBe(false);
    load.mockResolvedValueOnce([]);
    await act(async () => {
      await result.current.reload();
    });
    expect(result.current.items).toEqual([]);
  });

  it("reloads when the domain identity changes", async () => {
    const first = mock(async () => ["org-a"]);
    const second = mock(async () => ["org-b"]);
    const {result, rerender} = renderHook(({load}) => useEntityCollection(load), {initialProps: {load: first}});
    await waitFor(() => expect(result.current.items).toEqual(["org-a"]));
    rerender({load: second});
    await waitFor(() => expect(result.current.items).toEqual(["org-b"]));
  });

  it("tracks pending identity and preserves API success/error results", async () => {
    const {result} = renderHook(useEntityMutation);
    const pending = deferred<[null, {id: string}]>();
    let mutation!: Promise<[null, {id: string}]>;
    act(() => {
      mutation = result.current.run("user", () => pending.promise);
    });
    expect(result.current.pendingId).toBe("user");
    await act(async () => {
      pending.resolve([null, {id: "user"}]); await mutation;
    });
    expect(await mutation).toEqual([null, {id: "user"}]);
    expect(result.current.pendingId).toBeNull();
    const failure = [{message: "Forbidden"}, null];
    await act(async () => {
      expect(await result.current.run("user", async () => failure)).toBe(failure);
    });
    expect(result.current.pendingId).toBeNull();
  });
});
