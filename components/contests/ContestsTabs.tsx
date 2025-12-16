"use client";

import { Group, SegmentedControl } from "@mantine/core";
import { IconTrophy } from "@tabler/icons-react";
import { useRouter, useSearchParams } from "next/navigation";
import { usePageTransition } from "./ContestsPageWrapper";
import { useState, useEffect } from "react";
import { flushSync } from "react-dom";

type Props = {
  isAuthenticated: boolean;
};

export function ContestsTabs({ isAuthenticated }: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentView = searchParams.get("view") || "public";
  const { startTransition, pendingView, setPendingView, setIsPaginationTransition } = usePageTransition();
  const [localView, setLocalView] = useState<string>(currentView);
  
  // Sync local view with current view when URL changes
  useEffect(() => {
    setLocalView(currentView);
  }, [currentView]);
  
  // Show local view for immediate feedback
  const view = localView;

  const handleChange = (value: string) => {
    // Don't do anything if clicking on already active tab
    if (value === currentView) return;
    
    // Set local view IMMEDIATELY for instant UI feedback
    flushSync(() => {
      setLocalView(value);
    });
    
    const params = new URLSearchParams(searchParams);
    if (value === "user") {
      params.set("view", "user");
    } else {
      params.delete("view");
    }
    params.delete("page"); // Reset to page 1 on filter change
    params.delete("search"); // Reset search on filter change

    const query = params.toString();
    
    // Set pending view for skeleton display
    setPendingView(value);
    
    // Mark this as NON-pagination transition
    setIsPaginationTransition(false);
    
    startTransition(() => {
      router.push(`/contests${query ? `?${query}` : ""}`);
    });
  };

  return (
    <SegmentedControl
      value={view}
      onChange={handleChange}
      radius="md"
      size="md"
      data={[
        {
          value: "public",
          label: (
            <Group gap="xs" wrap="nowrap">
              <IconTrophy size={18} />
              <span>Публичные контесты</span>
            </Group>
          ),
        },
        {
          value: "user",
          label: (
            <Group gap="xs" wrap="nowrap">
              <IconTrophy size={18} />
              <span>Мои контесты</span>
            </Group>
          ),
          disabled: !isAuthenticated
        },
      ]}
    />
  );
}

