"use client";

import { TextInput } from "@mantine/core";
import { useDebouncedValue } from "@mantine/hooks";
import { IconSearch } from "@tabler/icons-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

export function AdminContestsSearchInput() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const initialSearch = searchParams.get("search") || "";
  const [search, setSearch] = useState(initialSearch);
  const [debouncedSearch] = useDebouncedValue(search, 300);

  useEffect(() => {
    const params = new URLSearchParams(searchParams);
    if (debouncedSearch) {
      params.set("search", debouncedSearch);
    } else {
      params.delete("search");
    }
    params.delete("page"); // Reset to page 1 on search change

    const query = params.toString();
    router.push(`/admin${query ? `?${query}` : ""}`);
  }, [debouncedSearch]);

  return (
    <TextInput
      placeholder="Поиск контестов..."
      leftSection={<IconSearch size={16} />}
      value={search}
      onChange={(e) => setSearch(e.currentTarget.value)}
      style={{ maxWidth: 400 }}
    />
  );
}

