import {Input} from "@mantine/core";
import {IconSearch} from "@tabler/icons-react";
import {useEffect, useRef, useState} from "react";
import {useNavigate, useSearchParams} from "react-router-dom";

import type {ReactNode} from "react";

export const UsersSearchInput = (): ReactNode => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const initialSearch = searchParams.get("search") || "";
  const [search, setSearch] = useState(initialSearch);
  const searchTimeoutRef = useRef<NodeJS.Timeout | undefined>(undefined);
  const isFirstRender = useRef(true);

  useEffect(() => {
    const urlSearch = searchParams.get("search") || "";
    if (urlSearch !== search) {
      setSearch(urlSearch);
    }
  }, [searchParams, search]);

  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }

    const urlSearch = searchParams.get("search") || "";
    if (search === urlSearch) {
      return;
    }

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      const params = new URLSearchParams(searchParams);
      params.delete("page");
      if (search) {
        params.set("search", search);
      } else {
        params.delete("search");
      }

      const query = params.toString();
      navigate(`/admin/users${query ? `?${query}` : ""}`);
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [search, navigate, searchParams]);

  return (
    <Input
      placeholder="Поиск по пользователям..."
      leftSection={<IconSearch size={16} />}
      value={search}
      onChange={(e) => setSearch(e.currentTarget.value)}
    />
  );
};
