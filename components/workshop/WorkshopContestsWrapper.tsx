"use client";

import { useState } from "react";
import type {
  PaginationModel as PaginationType,
} from "@contracts/core/v1";
import { usePageTransition } from "./WorkshopPageWrapper";
import { WorkshopContestsContentSkeleton } from "./WorkshopContestsContentSkeleton";
import { WorkshopContestsDataWrapper } from "./WorkshopContestsDataWrapper";
import type { ContestModel } from "@contracts/core/v1";
import { WorkshopContestsSearchInput } from "./WorkshopContestsSearchInput";
import { WorkshopProblemsContentSkeleton } from "./WorkshopProblemsContentSkeleton";
import { Stack } from "@mantine/core";
type Props = {
  contests: ContestModel[];
  pagination: PaginationType;
};

export function WorkshopContestsWrapper({
  contests,
  pagination
}: Props) {
  const [search, setSearch] = useState("");
  const { isPending, isPaginationTransition } = usePageTransition();
  console.log(isPaginationTransition);
  return (
    <>
      {isPending ? (!isPaginationTransition ? (
        <WorkshopProblemsContentSkeleton />
      ) : (
        <WorkshopContestsContentSkeleton />
      )
      ) : (
        <Stack gap="md">
          <WorkshopContestsSearchInput />
          <WorkshopContestsDataWrapper
            contests={contests}
            pagination={pagination}
            search={search}
          />
        </Stack>
      )}
    </>
  );
}