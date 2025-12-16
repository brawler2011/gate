"use client";

import type {
  ProblemsListItemModel,
  PaginationModel as PaginationType,
} from "@contracts/core/v1";
import { WorkshopProblemsSearchInput } from "./WorkshopProblemsSearchInput";
import { WorkshopProblemsContent } from "./WorkshopProblemsContent";
import { usePageTransition } from "./WorkshopPageWrapper";
import { useState } from "react";
import { WorkshopContestsContentSkeleton } from "./WorkshopContestsContentSkeleton";
import { WorkshopProblemsContentSkeleton } from "./WorkshopProblemsContentSkeleton";
import { Stack } from "@mantine/core";

type Props = {
  problems: ProblemsListItemModel[];
  pagination: PaginationType;
  isAuthenticated: boolean;
  owner?: string;
};

export function WorkshopProblemsWrapper({
  problems,
  pagination,
  isAuthenticated,
  owner,
}: Props) {
  const [search, setSearch] = useState("");
  const { isPending, isPaginationTransition } = usePageTransition();

  return (
    <>
      {isPending ? (!isPaginationTransition ? (
        <WorkshopContestsContentSkeleton />
      ) : (
        <WorkshopProblemsContentSkeleton />
      )
      ) : (
        <Stack gap="md">
          <WorkshopProblemsSearchInput value={search} onChange={setSearch} />
          <WorkshopProblemsContent
            problems={problems}
            pagination={pagination}
            isAuthenticated={isAuthenticated}
            owner={owner}
            search={search}
          />
        </Stack>
      )}
    </>
  );
}
