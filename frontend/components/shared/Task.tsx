"use client";

import {
  Box,
  Container,
  NavLink,
  Paper,
  Stack,
  Title,
} from "@mantine/core";
import Link from "next/link";
import React, {type ReactNode} from "react";

import {Problem} from "@/components/problems/Problem";
import {CreateSubmissionForm} from "@/components/submissions/CreateSubmissionForm";
import {RecentSubmissionsTable} from "@/components/submissions/RecentSubmissionsTable";
import {api} from "@/lib/api";
import {
  CONTEST_SIDEBAR_LEFT_WIDTH,
  CONTEST_SIDEBAR_RIGHT_WIDTH,
  LANGUAGE_MAP,
} from "@/lib/constants";
import {numberToLetters} from "@/lib/lib";

import type {
  ContestModel,
  ContestProblemListItemModel,
  ContestProblemModel,
  SubmissionsListItemModel,
  UserModel,
} from "@/contracts/core/v1";

type PageProps = {
  tasks: ContestProblemListItemModel[];
  contest: ContestModel;
  task: ContestProblemModel;
  submissions: SubmissionsListItemModel[];
  problemId: string;
  contestId: string;
  user: UserModel | null;
  wsUrl?: string;
  since?: number;
  isManager?: boolean;
};

const Task = ({
  tasks,
  contest,
  task,
  submissions,
  problemId,
  contestId: _contestId,
  user,
  wsUrl,
  since,
  isManager,
}: PageProps): ReactNode => {
  const onSubmit = async (
    submission: FormData,
    language: string,
  ): Promise<number | null> => {
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
      problemId,
      organizationLogin: contest.organization_login,
      contestLogin: contest.login,
      language: languageCode,
      requestBody: {
        submission: submissionContent,
      },
    });

    if (error) {
      console.error("Failed to create submission:", error);
      return null;
    }

    return response?.id ? 1 : null;
  };

  return (
    <Box maw="1920px" mx="auto" w="100%">
      <Box
        style={{
          display: "flex",
          gap: "16px",
          alignItems: "flex-start",
          paddingTop: "var(--mantine-spacing-md)",
          paddingBottom: "var(--mantine-spacing-md)",
          paddingLeft: "var(--mantine-spacing-md)",
          paddingRight: "var(--mantine-spacing-md)",
        }}
      >
        {/* Left Sidebar - скрыт на мобилках */}
        <Box style={{width: CONTEST_SIDEBAR_LEFT_WIDTH}} visibleFrom="sm">
          <Paper
            shadow="sm"
            radius="md"
            p="md"
            withBorder
            bg="var(--mantine-color-gray-light)"
            style={{
              maxHeight: "calc(100vh - 120px)",
              overflowY: "auto",
              position: "sticky",
              top: "80px",
            }}
          >
            <Stack gap="sm">
              <Title order={5} c="dimmed">
                Задачи
              </Title>
              <Stack gap="xs">
                {tasks.map((t, index) => {
                  const letter = numberToLetters(t.position);
                  const isCurrent = t.problem_id === problemId;
                  const isActive = isCurrent;

                  return (
                    <NavLink
                      key={t.problem_id || index}
                      component={Link}
                      href={`/${contest.organization_login}/contests/${contest.login}/problems/${letter}`}
                      label={`${letter}. ${t.title}`}
                      active={isActive}
                      styles={{
                        root: {
                          borderRadius: "var(--mantine-radius-sm)",
                          fontWeight: isActive ? 600 : 400,
                          backgroundColor: isActive
                            ? "light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))"
                            : "transparent",
                          color: isActive
                            ? "light-dark(var(--mantine-color-dark-9), var(--mantine-color-gray-0))"
                            : "light-dark(var(--mantine-color-dark-6), var(--mantine-color-gray-3))",
                          "&:hover": {
                            backgroundColor: isActive
                              ? "light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4))"
                              : "light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))",
                          },
                        },
                      }}
                    />
                  );
                })}
              </Stack>
            </Stack>
          </Paper>
        </Box>

        {/* Main Content */}
        <Box style={{flex: 1}}>
          <Container size="lg" px={0} mx={0} style={{maxWidth: "100%"}}>
            <Box pt="md">
              <Problem
                problem={task}
                letter={numberToLetters(task.position)}
                problemId={task.problem_id}
                isManager={isManager}
                orgLogin={contest.organization_login}
              />
            </Box>
          </Container>
        </Box>

        {/* Right Sidebar - скрыт на мобилках */}
        <Box
          visibleFrom="sm"
          style={{width: CONTEST_SIDEBAR_RIGHT_WIDTH}}
        >
          <Stack gap="md">
            <Paper
              shadow="sm"
              radius="md"
              p="md"
              withBorder
              bg="var(--mantine-color-gray-light)"
              style={{width: "100%"}}
            >
              <CreateSubmissionForm onSubmit={onSubmit} />
            </Paper>

            <RecentSubmissionsTable
              submissions={submissions}
              orgLogin={contest.organization_login}
              contestLogin={contest.login}
              contestId={contest.id}
              userId={user?.id}
              problemId={problemId}
              wsUrl={wsUrl}
              since={since}
            />
          </Stack>
        </Box>
      </Box>
    </Box>
  );
};

export {Task};
