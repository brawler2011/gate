"use client";

import {useCallback, useEffect, useState} from "react";

import {api} from "@/lib/api";

export const NOTIFICATIONS_UPDATED_EVENT = "gate:notifications_updated";

export const dispatchNotificationsUpdated = (): void => {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(NOTIFICATIONS_UPDATED_EVENT));
  }
};

type UseUnreadNotificationsCountResult = {
  unreadCount: number;
  refetch: () => Promise<void>;
};

export const useUnreadNotificationsCount = (
  isAuthenticated: boolean,
): UseUnreadNotificationsCountResult => {
  const [unreadCount, setUnreadCount] = useState<number>(0);

  const fetchCount = useCallback(async () => {
    if (!isAuthenticated) {
      setUnreadCount(0);
      return;
    }
    const [error, data] = await api.getUnreadNotificationsCount();
    if (!error && data) {
      setUnreadCount(data.count);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    fetchCount();

    if (!isAuthenticated) {
      return;
    }

    // Background polling every 45s
    const interval = setInterval(fetchCount, 45000);

    const handleUpdate = () => {
      fetchCount();
    };

    const handleFocus = () => {
      fetchCount();
    };

    window.addEventListener(NOTIFICATIONS_UPDATED_EVENT, handleUpdate);
    window.addEventListener("focus", handleFocus);

    return () => {
      clearInterval(interval);
      window.removeEventListener(NOTIFICATIONS_UPDATED_EVENT, handleUpdate);
      window.removeEventListener("focus", handleFocus);
    };
  }, [fetchCount, isAuthenticated]);

  return {unreadCount, refetch: fetchCount};
};
