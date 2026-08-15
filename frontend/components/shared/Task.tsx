"use client";

import {
  AppShellFooter,
  AppShellHeader,
  AppShellMain,
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
import {Layout} from "@/components/shared";
import {Footer} from "@/components/shared/Footer";
import {CreateSubmissionForm} from "@/components/submissions/CreateSubmissionForm";
import {RecentSubmissionsTable} from "@/components/submissions/RecentSubmissionsTable";
import {
  CONTEST_SIDEBAR_LEFT_WIDTH,
  CONTEST_SIDEBAR_RIGHT_WIDTH,
  LANGUAGE_MAP,
} from "@/lib/constants";
import {numberToLetters} from "@/lib/lib";
import {createSolution} from "@/lib/workshop";

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
  header: React.ReactNode;
  wsUrl?: string;
  since?: number;
};

const Task = ({
  tasks,
  contest,
  task,
  submissions,
  problemId,
  contestId,
  user,
  header,
  wsUrl,
  since,
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

    const [error, response] = await createSolution(
      problemId,
      contestId,
      languageCode,
      submission,
    );

    if (error) {
      console.error("Failed to create submission:", error);
      return null;
    }

    return response?.id ? 1 : null;
  };

  return (
    <Layout paddingConfig="0">
      <AppShellHeader>{header}</AppShellHeader>
      <AppShellMain>
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
                style={{width: "100%"}}
              >
                <Stack w="100%" gap="xs">
                  <Link
                    href={`/contests/${contest.id}`}
                    style={{textDecoration: "none"}}
                  >
                    <Title
                      c="var(--mantine-color-text)"
                      order={4}
                      ta="center"
                      style={{cursor: "pointer"}}
                    >
                      {contest.title}
                    </Title>
                  </Link>
                  <Stack gap={2}>
                    {tasks.map((item) => {
                      const isActive = item.problem_id === task.problem_id;
                      return (
                        <NavLink
                          key={item.problem_id}
                          component={Link}
                          href={`/contests/${contest.id}/problems/${item.problem_id}`}
                          label={`${numberToLetters(item.position)}. ${item.title}`}
                          active={isActive}
                          c="var(--mantine-color-text)"
                          styles={{
                            root: {
                              paddingLeft: 8,
                              paddingRight: 8,
                              borderRadius: "var(--mantine-radius-sm)",
                              fontWeight: isActive ? 600 : 400,
                              backgroundColor: isActive
                                ? "light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))"
                                : "transparent",
                              "&:hover": {
                                backgroundColor: isActive
                                  ? "light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-4))"
                                  : "light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))",
                              },
                            },
                            label: {fontSize: "0.875rem"},
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
      </AppShellMain>
      <AppShellFooter withBorder={false}>
        <Footer />
      </AppShellFooter>
    </Layout>
  );
};

export {Task};
