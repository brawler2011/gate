"use client";

import { TextInput } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

export function AdminBlogsSearchInput() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const initialSearch = searchParams.get("search") || "";
  const [search, setSearch] = useState(initialSearch);
  const searchTimeoutRef = useRef<NodeJS.Timeout>();
  const isFirstRender = useRef(true);

  // Sync state with URL when searchParams change externally (e.g., browser back)
  useEffect(() => {
    const urlSearch = searchParams.get("search") || "";
    if (urlSearch !== search) {
      setSearch(urlSearch);
    }
  }, [searchParams]);

  // Only update URL when user actually types (not on initial render or URL change)
  useEffect(() => {
    // Skip initial render
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }

    // Skip if search matches current URL (was set from URL sync)
    const urlSearch = searchParams.get("search") || "";
    if (search === urlSearch) {
      return;
    }

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      const params = new URLSearchParams(searchParams);
      params.delete("page"); // Reset to first page on search
      if (search) {
        params.set("search", search);
      } else {
        params.delete("search");
      }

      const query = params.toString();
      router.push(`/admin${query ? `?${query}` : ""}`);
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [search]);

  return (
    <TextInput
      placeholder="Поиск постов..."
      leftSection={<IconSearch size={16} />}
      value={search}
      onChange={(e) => setSearch(e.currentTarget.value)}
      style={{ maxWidth: 400 }}
    />
  );
}


