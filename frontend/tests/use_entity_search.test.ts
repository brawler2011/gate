import "./setup-dom";

import {act, cleanup, renderHook, waitFor} from "@testing-library/react";
import {afterAll, beforeAll, describe, expect, it} from "bun:test";

import {useEntitySearch} from "@/hooks/useEntitySearch";

import {setupDOMEnvironment} from "./test-utils";

describe("useEntitySearch Hook test", () => {
  let cleanupDOM: () => void;

  beforeAll(() => {
    cleanupDOM = setupDOMEnvironment();
  });

  afterAll(async () => {
    cleanup();
    await new Promise((r) => setTimeout(r, 10));
    cleanupDOM();
  });

  it("handles search queries and selection", async () => {
    const mockTeams = [
      {id: "t1", name: "Alpha"},
      {id: "t2", name: "Beta"},
    ];

    const searchFn = async (query: string) => {
      return mockTeams.filter((t) => t.name.toLowerCase().includes(query.toLowerCase()));
    };

    const {result, unmount} = renderHook(() =>
      useEntitySearch({
        searchFn,
        debounceMs: 0,
        minQueryLength: 1,
      })
    );

    expect(result.current.searchQuery).toBe("");
    expect(result.current.results).toEqual([]);

    await act(async () => {
      result.current.setSearchQuery("alp");
    });

    await act(async () => {
      await new Promise((r) => setTimeout(r, 50));
    });

    expect(result.current.results.length).toBe(1);
    expect(result.current.results[0].label).toBe("Alpha");

    await act(async () => {
      result.current.selectOption("t1");
    });

    expect(result.current.selectedId).toBe("t1");

    await act(async () => {
      result.current.reset();
    });

    expect(result.current.searchQuery).toBe("");
    expect(result.current.selectedId).toBeNull();

    unmount();
  });

  it("uses the latest inline search and mapping functions after rerender", async () => {
    const items = [{id: "1", name: "Alpha"}, {id: "2", name: "Beta"}];
    const {result, rerender, unmount} = renderHook(({prefix}) => useEntitySearch({
      searchFn: async (query) => items.filter((item) => item.name.includes(query)),
      mapToItem: (item) => ({value: item.id, label: `${prefix}: ${item.name}`}),
      debounceMs: 0,
      minQueryLength: 2,
    }), {initialProps: {prefix: "first"}});
    act(() => {
      result.current.setSearchQuery("Al");
    });
    await waitFor(() => expect(result.current.results).toEqual([{value: "1", label: "first: Alpha"}]));
    rerender({prefix: "second"});
    act(() => {
      result.current.setSearchQuery("Be");
    });
    await waitFor(() => expect(result.current.results).toEqual([{value: "2", label: "second: Beta"}]));
    unmount();
  });

  it("discards stale out-of-order async responses (race condition prevention)", async () => {
    let resolveFirst!: (val: {id: string; name: string}[]) => void;
    let resolveSecond!: (val: {id: string; name: string}[]) => void;

    const firstPromise = new Promise<{id: string; name: string}[]>((res) => {
      resolveFirst = res;
    });
    const secondPromise = new Promise<{id: string; name: string}[]>((res) => {
      resolveSecond = res;
    });

    const searchFn = async (query: string) => {
      if (query === "first") {
        return firstPromise;
      }
      if (query === "second") {
        return secondPromise;
      }
      return [];
    };

    const {result, unmount} = renderHook(() =>
      useEntitySearch({
        searchFn,
        mapToItem: (item) => ({value: item.id, label: item.name}),
        debounceMs: 0,
        minQueryLength: 1,
      })
    );

    // Trigger first search
    await act(async () => {
      result.current.setSearchQuery("first");
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });

    // Trigger second search while first is still in-flight
    await act(async () => {
      result.current.setSearchQuery("second");
    });
    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });

    // Resolve second (latest) search first
    await act(async () => {
      resolveSecond([{id: "second", name: "Result for second"}]);
      await new Promise((r) => setTimeout(r, 10));
    });

    expect(result.current.results.length).toBe(1);
    expect(result.current.results[0].value).toBe("second");

    // Resolve first (older) search afterwards
    await act(async () => {
      resolveFirst([{id: "first", name: "Result for first"}]);
      await new Promise((r) => setTimeout(r, 10));
    });

    // Results must NOT be overwritten by the stale first response
    expect(result.current.results.length).toBe(1);
    expect(result.current.results[0].value).toBe("second");
    expect(result.current.results[0].label).toBe("Result for second");

    unmount();
  });

  it("clears search results when query becomes shorter than minQueryLength", async () => {
    const mockItems = [{id: "1", name: "Alpha"}];
    const searchFn = async (_query: string) => mockItems;

    const {result, unmount} = renderHook(() =>
      useEntitySearch({
        searchFn,
        debounceMs: 0,
        minQueryLength: 3,
      })
    );

    await act(async () => {
      result.current.setSearchQuery("Alph");
    });

    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });

    expect(result.current.results.length).toBe(1);

    // Shorten query below minQueryLength
    await act(async () => {
      result.current.setSearchQuery("Al");
    });

    await act(async () => {
      await new Promise((r) => setTimeout(r, 20));
    });

    expect(result.current.results).toEqual([]);
    expect(result.current.rawResults).toEqual([]);

    unmount();
  });
});
