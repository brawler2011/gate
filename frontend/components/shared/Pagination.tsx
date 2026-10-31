import {Pagination} from "@mantine/core";
import {Link} from "react-router-dom";

import type * as corev1 from "@/contracts/core/v1/models";
import type {ReactNode} from "react";

interface RoutePaginationProps {
  pagination: corev1.PaginationModel;
  baseUrl: string;
  queryParams?: Record<string, string | number | undefined>;
}

const RoutePagination = ({
  pagination,
  baseUrl,
  queryParams = {},
}: RoutePaginationProps): ReactNode => {
  // Helper function to build query string
  const buildQueryString = (
    params: Record<string, string | number | undefined>,
  ) => {
    const validParams = Object.entries(params)
      .filter(([_key, value]) => value !== undefined && value !== "")
      .map(([key, _value]) => `${key}=${encodeURIComponent(_value!)}`);
    return validParams.length > 0 ? `?${validParams.join("&")}` : "";
  };

  const getItemProps = (page: number) => ({
    component: Link,
    to: `${baseUrl}${buildQueryString({...queryParams, page})}`,
  });

  const getControlProps = (control: "first" | "previous" | "last" | "next") => {
    if (control === "next") {
      const nextPage =
        pagination.page === pagination.total
          ? pagination.page
          : pagination.page + 1;
      return {
        component: Link,
        to: `${baseUrl}${buildQueryString({...queryParams, page: nextPage})}`,
      };
    }

    if (control === "previous") {
      const prevPage =
        pagination.page === 1 ? pagination.page : pagination.page - 1;
      return {
        component: Link,
        to: `${baseUrl}${buildQueryString({...queryParams, page: prevPage})}`,
      };
    }

    return {};
  };

  return (
    <Pagination
      total={pagination.total}
      value={pagination.page}
      getItemProps={getItemProps}
      getControlProps={getControlProps}
    />
  );
};

export {RoutePagination};
