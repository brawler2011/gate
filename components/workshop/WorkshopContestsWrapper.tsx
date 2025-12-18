"use client";

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
  search?: string;
};

export function WorkshopContestsWrapper({
  contests,
  pagination,
  search = ""
}: Props) {
  const { isPending, isPaginationTransition } = usePageTransition();

  return (
    <>
      {/* { THIS IS NOT MISTAKE. IT IS THE CORRECT WAY TO HANDLE THE PAGINATION TRANSITION. COMPONENTS SHOULD BE INVERTED } */
      isPending ? (!isPaginationTransition ? ( 
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