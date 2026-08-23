"use client";

import {useDebouncedValue} from "@mantine/hooks";
import {useCallback, useEffect, useRef, useState} from "react";

type EntitySearchItem = {
  value: string;
  label: string;
  data?: unknown;
};

export interface UseEntitySearchOptions<T> {
  searchFn: (query: string) => Promise<T[]>;
  mapToItem?: (item: T) => EntitySearchItem;
  debounceMs?: number;
  minQueryLength?: number;
}

export interface UseEntitySearchResult<T> {
  searchQuery: string;
  setSearchQuery: (query: string) => void;
  debouncedQuery: string;
  results: EntitySearchItem[];
  rawResults: T[];
  searching: boolean;
  selectedId: string | null;
  selectOption: (id: string | null) => void;
  reset: () => void;
}

export const useEntitySearch = <T,>({
  searchFn,
  mapToItem,
  debounceMs = 300,
  minQueryLength = 2,
}: UseEntitySearchOptions<T>): UseEntitySearchResult<T> => {
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, debounceMs);
  const [results, setResults] = useState<EntitySearchItem[]>([]);
  const [rawResults, setRawResults] = useState<T[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const searchFnRef = useRef(searchFn);
  const mapToItemRef = useRef(mapToItem);
  const requestIdRef = useRef(0);

  useEffect(() => {
    searchFnRef.current = searchFn;
    mapToItemRef.current = mapToItem;
  }, [searchFn, mapToItem]);

  const performSearch = useCallback(
    async (query: string) => {
      const trimmed = query?.trim() ?? "";
      const currentRequestId = ++requestIdRef.current;

      if (!trimmed || trimmed.length < minQueryLength) {
        setResults((prev) => (prev.length === 0 ? prev : []));
        setRawResults((prev) => (prev.length === 0 ? prev : []));
        setSearching(false);
        return;
      }

      setSearching(true);
      try {
        const items = await searchFnRef.current(trimmed);
        if (currentRequestId !== requestIdRef.current) {
          return;
        }

        setRawResults(items);
        if (mapToItemRef.current) {
          setResults(items.map(mapToItemRef.current));
        } else {
          setResults(
            items.map((item) => {
              const obj = item as Record<string, unknown>;
              return {
                value: String(obj.id || obj.value || ""),
                label: String(obj.name || obj.username || obj.title || obj.label || ""),
                data: item,
              };
            })
          );
        }
      } catch (error) {
        if (currentRequestId === requestIdRef.current) {
          console.error("Entity search failed:", error);
          setResults((prev) => (prev.length === 0 ? prev : []));
          setRawResults((prev) => (prev.length === 0 ? prev : []));
        }
      } finally {
        if (currentRequestId === requestIdRef.current) {
          setSearching(false);
        }
      }
    },
    [minQueryLength]
  );

  useEffect(() => {
    performSearch(debouncedQuery);
  }, [debouncedQuery, performSearch]);

  const selectOption = useCallback((id: string | null) => {
    setSelectedId(id);
  }, []);

  const reset = useCallback(() => {
    requestIdRef.current++;
    setSearchQuery("");
    setResults((prev) => (prev.length === 0 ? prev : []));
    setRawResults((prev) => (prev.length === 0 ? prev : []));
    setSearching(false);
    setSelectedId(null);
  }, []);

  return {
    searchQuery,
    setSearchQuery,
    debouncedQuery,
    results,
    rawResults,
    searching,
    selectedId,
    selectOption,
    reset,
  };
};
