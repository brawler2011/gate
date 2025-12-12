"use client";

import { Group, SegmentedControl } from "@mantine/core";
import { IconNews, IconTrophy, IconUsers } from "@tabler/icons-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useState, useEffect } from "react";
import { flushSync } from "react-dom";

export function AdminTabs() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentView = searchParams.get("view") || "users";
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
    if (value === "contests") {
      params.set("view", "contests");
    } else if (value === "blogs") {
      params.set("view", "blogs");
    } else {
      params.delete("view");
    }
    params.delete("page"); // Reset to page 1 on filter change
    params.delete("search"); // Reset search on filter change

    const query = params.toString();

    router.push(`/admin${query ? `?${query}` : ""}`);
  };

  return (
    <SegmentedControl
      value={view}
      onChange={handleChange}
      radius="md"
      size="md"
      data={[
        {
          value: "users",
          label: (
            <Group gap="xs" wrap="nowrap">
              <IconUsers size={18} />
              <span>Пользователи</span>
            </Group>
          ),
        },
        {
          value: "contests",
          label: (
            <Group gap="xs" wrap="nowrap">
              <IconTrophy size={18} />
              <span>Контесты</span>
            </Group>
          ),
        },
        {
          value: "blogs",
          label: (
            <Group gap="xs" wrap="nowrap">
              <IconNews size={18} />
              <span>Блоги</span>
            </Group>
          ),
        },
      ]}
    />
  );
}

