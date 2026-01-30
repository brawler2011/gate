import { Center, SimpleGrid, Text } from "@mantine/core";
import type { ProblemsListItemModel } from "@contracts/core/v1";
import { ProblemCard } from '@/components/problems/ProblemCard';

type Props = {
  problems: ProblemsListItemModel[];
  isAuthenticated: boolean;
  owner?: string;
};

export function WorkshopProblemsGrid({ problems, isAuthenticated, owner }: Props) {
  if (problems.length === 0) {
    return (
      <Center py="xl">
        <Text c="dimmed">Задачи не найдены</Text>
      </Center>
    );
  }

  // Show edit button only in "My Problems" section
  const showEditButton = isAuthenticated && owner === "me";

  return (
    <SimpleGrid
      cols={{ base: 1, xs: 2, sm: 2, md: 3, lg: 4, xl: 4 }}
      spacing={{ base: "xs", sm: "sm", md: "md" }}
    >
      {problems.map((problem) => (
        <ProblemCard
          key={problem.id}
          problem={problem}
          showEditButton={showEditButton}
        />
      ))}
    </SimpleGrid>
  );
}

