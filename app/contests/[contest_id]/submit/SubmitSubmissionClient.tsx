"use client";

import { CreateSubmissionForm } from "@/components/CreateSubmissionForm";
import { numberToLetters } from "@/lib/lib";
import { Box, Paper, Select, Stack } from "@mantine/core";
import { useState } from "react";
import { useRouter } from "next/navigation";
import type { ContestModel, ContestProblemListItemModel } from "../../../../../contracts/core/v1";
import type { SessionUser } from "@/lib/auth";
import { submitSubmission } from "./actions";

type Props = {
  contest: ContestModel;
  problems: ContestProblemListItemModel[];
  user: SessionUser;
};

export function SubmitSubmissionClient({ contest, problems, user }: Props) {
  const router = useRouter();
  const [selectedProblemId, setSelectedProblemId] = useState<string | null>(
    problems.length > 0 ? problems[0].problem_id : null
  );
  const [isSubmitted, setIsSubmitted] = useState(false);

  const problemOptions = problems.map((problem) => ({
    value: problem.problem_id,
    label: `${numberToLetters(problem.position)}. ${problem.title}`,
  }));

  const handleSubmit = async (submission: FormData, language: string) => {
    if (!selectedProblemId) {
      console.error("No problem selected");
      return null;
    }

    const result = await submitSubmission(
      selectedProblemId,
      contest.id,
      submission,
      language
    );

    if (result) {
      // Mark as submitted to disable form
      setIsSubmitted(true);
      // Redirect to "Мои посылки" page after successful submission
      router.push(`/mysubmissions?contestId=${contest.id}&order=desc&userId=${user?.id}`);
    }

    return result;
  };

  if (problems.length === 0) {
    return (
      <Box style={{ maxWidth: 740, margin: "0 auto" }}>
        <Stack gap="lg">
          <p>В этом контесте пока нет задач</p>
        </Stack>
      </Box>
    );
  }

  return (
    <Box style={{ maxWidth: 740, margin: "0 auto"}}>
      <Paper 
        shadow="sm" 
        radius="md" 
        p="md" 
        withBorder 
        bg="var(--mantine-color-gray-light)"
      >
        <CreateSubmissionForm 
          onSubmit={handleSubmit}
          large={true}
          disabled={isSubmitted}
          problemSelect={
            <Select
              placeholder="Выберите задачу для отправки решения"
              data={problemOptions}
              value={selectedProblemId}
              onChange={setSelectedProblemId}
              allowDeselect={false}
              disabled={isSubmitted}
              style={{ width: 200 }}
            />
          }
        />
      </Paper>
    </Box>
  );
}
