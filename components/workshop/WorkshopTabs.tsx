"use client";

import { Group, SegmentedControl } from "@mantine/core";
import { IconPuzzle, IconTrophy } from "@tabler/icons-react";
import { useRouter, useSearchParams } from "next/navigation";
import { usePageTransition } from "./WorkshopPageWrapper";
import { useState, useEffect } from "react";
import { flushSync } from "react-dom";
import { APP_COLORS } from "@/lib/theme/colors";

type Props = {
  isAuthenticated: boolean;
};

export function WorkshopTabs({ isAuthenticated }: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentView = searchParams.get("view") || "contests";
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
    if (value === "problems") {
      params.set("view", "problems");
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
      router.push(`/workshop${query ? `?${query}` : ""}`);
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
          value: "contests",
          label: (
            <Group gap="xs" wrap="nowrap">
              <IconTrophy size={18} />
              <span>Мои контесты</span>
            </Group>
          ),
          disabled: !isAuthenticated
        },
        {
          value: "problems",
          label: (
            <Group gap="xs" wrap="nowrap">
              <IconPuzzle size={18} />
              <span>Мои задачи</span>
            </Group>
          ),
          disabled: !isAuthenticated
        },
      ]}
    />
  );
}
