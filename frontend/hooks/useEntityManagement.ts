"use client";

import {useCallback, useEffect, useState} from "react";

// Callers keep domain-specific API calls and error presentation.
export const useEntityCollection = <T,>(
  load: () => Promise<T[] | undefined>,
  initiallyLoading = true
): {items: T[]; loading: boolean; reload: () => Promise<void>} => {
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(initiallyLoading);
  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const data = await load();
      if (data) {
        setItems(data);
      }
    } finally {
      setLoading(false);
    }
  }, [load]);

  useEffect(() => {
    void reload();
  }, [reload]);

  return {items, loading, reload};
};

export const useEntityMutation = (): {
  pendingId: string | null;
  run: <T>(id: string, mutation: () => Promise<T>) => Promise<T>;
} => {
  const [pendingId, setPendingId] = useState<string | null>(null);
  const run = useCallback(async <T,>(id: string, mutation: () => Promise<T>): Promise<T> => {
    setPendingId(id);
    try {
      return await mutation();
    } finally {
      setPendingId(null);
    }
  }, []);

  return {pendingId, run};
};
