"use client";

import {Box, Paper, Select, Stack} from "@mantine/core";
import {useRouter} from "next/navigation";
import {useState} from "react";

import {CreateSubmissionForm} from "@/components/submissions/CreateSubmissionForm";
import {api} from "@/lib/api";
import {LANGUAGE_MAP} from "@/lib/constants";
import {numberToLetters} from "@/lib/lib";

import classes from "./SubmitSubmissionClient.module.css";

import type {
  ContestModel,
  ContestProblemListItemModel,
  UserModel,
} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type Props = {
  contest: ContestModel;
  problems: ContestProblemListItemModel[];
  user: UserModel | null;
};

export const SubmitSubmissionClient = ({contest, problems, user}: Props): ReactNode => {
  const router = useRouter();
  const [selectedProblemId, setSelectedProblemId] = useState<string | null>(
    problems.length > 0 ? problems[0].problem_id : null,
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

    const languageCode = LANGUAGE_MAP[language];
    if (!languageCode) {
      console.error("Invalid language:", language);
      return null;
    }

    const submissionData = submission.get("submission");
    let submissionContent = "";
    if (submissionData instanceof File) {
      submissionContent = await submissionData.text();
    } else if (typeof submissionData === "string") {
      submissionContent = submissionData;
    }

    const [error, response] = await api.createSubmission({
      problemId: selectedProblemId,
      contestId: contest.id,
      language: languageCode,
      requestBody: {
        submission: submissionContent,
      },
    });

    if (error) {
      console.error("Failed to create submission:", error);
      return null;
    }

    const result = response?.id ? 1 : null;

    if (result) {
      // Mark as submitted to disable form
      setIsSubmitted(true);
      // Redirect to "Мои посылки" page after successful submission
      router.push(
        `/${contest.organization_login}/contests/${contest.id}/mysubmissions?order=desc&userId=${user?.id}`,
      );
    }

    return result;
  };

  if (problems.length === 0) {
    return (
      <Box style={{maxWidth: 740, margin: "0 auto"}}>
        <Stack gap="lg">
          <p>В этом контесте пока нет задач</p>
        </Stack>
      </Box>
    );
  }

  return (
    <Box style={{maxWidth: "100%", margin: "0 auto"}}>
      <Paper
        shadow="sm"
        p="md"
        withBorder
        bg="light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))"
        style={{
          borderRadius: "var(--mantine-radius-md)",
          borderColor:
            "light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))",
        }}
      >
        <CreateSubmissionForm
          onSubmit={handleSubmit}
          large
          disabled={isSubmitted}
          problemSelect={
            <Select
              placeholder="Выберите задачу"
              variant="subtle"
              data={problemOptions}
              value={selectedProblemId}
              onChange={setSelectedProblemId}
              allowDeselect={false}
              disabled={isSubmitted}
              classNames={{input: classes.problemSelectInput}}
              style={{
                width: `${(problemOptions.find((o) => o.value === selectedProblemId)?.label.length || 10) + 3}ch`,
              }}
              comboboxProps={{position: "bottom-start"}}
              styles={{dropdown: {minWidth: "max-content"}}}
            />
          }
        />
      </Paper>
    </Box>
  );
};
