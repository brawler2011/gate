"use client";

import { useState } from "react";
import type {
  PaginationModel as PaginationType,
} from "@contracts/core/v1";
import { usePageTransition } from "./ContestsPageWrapper";
import { ContestsContentSkeleton } from "./ContestsContentSkeleton";
import { PublicContestsDataWrapper } from "./PublicContestsDataWrapper";
import type { ContestModel } from "@contracts/core/v1";
import { ContestsSearchInput } from "./ContestsSearchInput";
import { Stack } from "@mantine/core";

type Props = {
  contests: ContestModel[];
  pagination: PaginationType;
};

export function PublicContestsWrapper({
  contests,
  pagination
}: Props) {
  const [search, setSearch] = useState("");
  const { isPending, isPaginationTransition } = usePageTransition();

  return (
    <>
      {isPending ? (
        <ContestsContentSkeleton />
      ) : (
        <Stack gap="md">
          <ContestsSearchInput />
          <PublicContestsDataWrapper
            contests={contests}
            pagination={pagination}
            search={search}
          />
        </Stack>
      )}
    </>
  );
}

