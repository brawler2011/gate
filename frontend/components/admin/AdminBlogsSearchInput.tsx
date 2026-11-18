import {TextInput} from "@mantine/core";
import {IconSearch} from "@tabler/icons-react";
import {useEffect, useRef, useState} from "react";
import {useNavigate, useSearchParams} from "react-router-dom";

import type {ReactNode} from "react";

export const AdminBlogsSearchInput = (): ReactNode => {
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
    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, []);

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
      navigate(`/admin/blogs${query ? `?${query}` : ""}`);
    }, 300);
  }, [search, searchParams, navigate]);

  return (
    <TextInput
      placeholder="Поиск постов..."
      leftSection={<IconSearch size={16} />}
      value={search}
      onChange={(e) => setSearch(e.currentTarget.value)}
      style={{maxWidth: 400}}
    />
  );
};
