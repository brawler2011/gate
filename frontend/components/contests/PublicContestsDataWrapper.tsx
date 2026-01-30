"use client";

import type {
  ContestModel,
  PaginationModel as PaginationType,
} from "@contracts/core/v1";
import { ContestsTable } from './ContestsTable';
import { NextPagination } from '@/components/shared/Pagination';
import { Center, Text } from "@mantine/core";
import { useSearchParams } from "next/navigation";

type Props = {
  contests: ContestModel[];
  pagination: PaginationType;
  search: string;
};

export function PublicContestsDataWrapper({ contests, pagination, search }: Props) {
  const searchParams = useSearchParams();
  const totalPages = pagination.total || 1;

  if (contests.length === 0) {
    return (
      <>
        <Center py="xl">
          <Text c="dimmed">
            {searchParams.get("search")
              ? "Контесты по вашему запросу не найдены"
              : "Публичных контестов пока нет"}
          </Text>
        </Center>
      </>
    );
  }

  return (
    <>
      <ContestsTable contests={contests} />
      {totalPages > 1 && (
        <Center>
          <NextPagination
            pagination={pagination}
            baseUrl="/contests"
            queryParams={{ search }}
          />
        </Center>
      )}
    </>
  );
}

