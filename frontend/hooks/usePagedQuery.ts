import {useQuery} from "@tanstack/react-query";
import {useEffect, useState} from "react";

import type {PaginationModel} from "@/contracts/core/v1/schema";
import type {ApiError} from "@/lib/api";
import type {UseQueryResult} from "@tanstack/react-query";

type PagedQuery<T> = UseQueryResult<T, ApiError> & {page: number; setPage: (page: number) => void};

// Reset on entity changes and keep the page valid when a mutation removes its last item.
export const usePagedQuery = <T extends {pagination: PaginationModel}>(
  key: readonly unknown[],
  fetchPage: (page: number, pageSize: number) => Promise<T>,
  pageSize = 50,
): PagedQuery<T> => {
  const scope = JSON.stringify(key);
  const [selection, setSelection] = useState({scope, page: 1});
  const page = selection.scope === scope ? selection.page : 1;
  const setPage = (page: number) => setSelection({scope, page});
  const query = useQuery<T, ApiError>({queryKey: [...key, page, pageSize], queryFn: () => fetchPage(page, pageSize)});
  const lastPage = Math.max(1, query.data?.pagination.total_pages ?? page);
  useEffect(() => {
    if (page > lastPage) {
      setSelection({scope, page: lastPage});
    }
  }, [page, lastPage, scope]);
  return {...query, page, setPage};
};
