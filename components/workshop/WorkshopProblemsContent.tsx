"use client";

import { Stack } from "@mantine/core";
import type {
  PaginationModel as PaginationType,
  ProblemsListItemModel,
} from "@contracts/core/v1";
import { NextPagination } from '@/components/shared/Pagination';
import { WorkshopProblemsGrid } from "./WorkshopProblemsGrid";

type Props = {
  problems: ProblemsListItemModel[];
  pagination: PaginationType;
  isAuthenticated: boolean;
  owner?: string;
  search: string;
};

export function WorkshopProblemsContent({
  problems,
  pagination,
  isAuthenticated,
  owner,
  search,
}: Props) {
  // Local search filtering (client-side)
  const filteredProblems = problems.filter((problem) =>
    problem.title?.toLowerCase().includes(search.toLowerCase())
  );

  const queryParams: Record<string, string | undefined> = {};
  queryParams.view = "problems";
  if (owner) queryParams.owner = owner;

  return (
    <>
      <WorkshopProblemsGrid 
        problems={filteredProblems} 
        isAuthenticated={isAuthenticated}
        owner={owner}
      />
      {pagination.total > 1 && (
        <Stack align="center">
          <NextPagination
            pagination={pagination}
            baseUrl="/workshop"
            queryParams={queryParams}
          />
        </Stack>
      )}
    </>
  );
}
