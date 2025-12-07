"use client";

import React from 'react';
import {
    Anchor,
    AppShellFooter,
    AppShellHeader,
    AppShellMain,
    Box,
    Container,
    NavLink,
    Paper,
    Stack,
    Title
} from "@mantine/core";
import {CreateSubmissionForm} from "@/components/CreateSubmissionForm";
import Link from "next/link";
import type {ContestProblemListItemModel, ContestModel, ContestProblemModel, SubmissionsListItemModel} from "../../contracts/core/v1";
import {Problem} from "@/components/Problem";
import {numberToLetters} from '@/lib/lib';
import {CONTEST_SIDEBAR_LEFT_WIDTH, CONTEST_SIDEBAR_RIGHT_WIDTH} from "@/lib/constants";
import {Layout} from "@/components/Layout";
import {Footer} from "@/components/Footer";
import {RecentSubmissionsTable} from "@/components/RecentSubmissionsTable";
import {submitSubmission} from "@/app/contests/[contest_id]/problems/[problem_id]/actions";
import {ContestHotbar} from "@/components/ContestHotbar";
import {ContestInfoPanel} from "@/components/ContestInfoPanel";
import type {SessionUser} from "@/lib/auth";
import type {ContestRole} from "@/lib/contest-role";

type PageProps = {
    tasks: ContestProblemListItemModel[]
    contest: ContestModel,
    task: ContestProblemModel,
    submissions: SubmissionsListItemModel[],
    problemId: string,
    contestId: string,
    user: SessionUser,
    contestRole: { role: ContestRole } | null,
    header: React.ReactNode,
    wsUrl?: string
}

const Task = ({tasks, contest, task, submissions, problemId, contestId, user, contestRole, header, wsUrl}: PageProps) => {
    const onSubmit = async (
        submission: FormData,
        language: string
    ): Promise<number | null> => {
        // WebSocket will automatically update the submissions list
        return await submitSubmission(problemId, contestId, submission, language);
    };

    return (
        <Layout paddingConfig="0">
            <AppShellHeader>
                {header}
            </AppShellHeader>
            <AppShellMain>
                <Box maw="1920px" mx="auto" w="100%">
            <Box style={{ display: 'flex', gap: '16px', alignItems: 'flex-start', paddingTop: 'var(--mantine-spacing-md)', paddingBottom: 'var(--mantine-spacing-md)', paddingRight: 'var(--mantine-spacing-md)' }}>
                    {/* Left Sidebar - скрыт на мобилках */}
                    <Box 
                        style={{ width: CONTEST_SIDEBAR_LEFT_WIDTH }}
                        visibleFrom="sm"
                    >
                        <Paper 
                            shadow="none" 
                            radius="md" 
                            px={0}
                            py="md"
                            withBorder 
                            bg="transparent"
                            style={{ 
                                borderColor: 'var(--mantine-color-dark-5)',
                                borderLeft: 'none',
                                borderTopLeftRadius: 0,
                                borderBottomLeftRadius: 0
                            }}
                        >
                            <Stack w="100%" gap="xs">
                                <Anchor component={Link} href={`/contests/${contest.id}`} c="var(--mantine-color-bright)">
                                    <Title c="var(--mantine-color-text)" order={4} ta="center">
                                        Задачи
                                    </Title>
                                </Anchor>
                                <Stack gap={0}>
                                    {tasks.map((item) => (
                                        <NavLink
                                            key={item.problem_id}
                                            component={Link}
                                            href={`/contests/${contest.id}/problems/${item.problem_id}`}
                                            label={`${numberToLetters(item.position)}. ${item.title}`}
                                            active={item.problem_id === task.problem_id}
                                            color="gray"
                                            variant="light"
                                            c="var(--mantine-color-text)"
                                            styles={{
                                                root: { 
                                                    paddingLeft: 8, 
                                                    paddingRight: 8,
                                                    '&:hover': {
                                                        backgroundColor: 'var(--mantine-color-dark-5)'
                                                    }
                                                },
                                                label: { fontSize: '0.875rem' }
                                            }}
                                        />
                                    ))}
                                </Stack>
                            </Stack>
                        </Paper>
                    </Box>

                    {/* Main Content */}
                    <Box style={{ flex: 1 }}>
                        <Container 
                            size="lg"
                            px={0}
                            mx={0}
                            style={{ maxWidth: '100%' }}
                        >
                            <ContestHotbar 
                                contest={contest}
                                user={user}
                                contestRole={contestRole}
                                align="left"
                            >
                                <Box pt="md">
                                    <Problem problem={task} letter={numberToLetters(task.position)}/>
                                </Box>
                            </ContestHotbar>
                        </Container>
                    </Box>

                    {/* Right Sidebar - скрыт на мобилках */}
                    <Box 
                        style={{ marginRight: '16px' }}
                        visibleFrom="sm"
                    >
                        <Stack gap="md">
                            {/* Contest Info Panel */}
                            <ContestInfoPanel 
                                contest={contest}
                                user={user}
                                contestRole={contestRole}
                                width="100%"
                            />
                            
                            {/* Submission Form and Recent Submissions */}
                            <Paper 
                                shadow="sm" 
                                radius="md" 
                                p="md" 
                                withBorder 
                                bg="var(--mantine-color-gray-light)"
                                style={{ width: CONTEST_SIDEBAR_RIGHT_WIDTH }}
                            >
                                <Stack>
                                    <CreateSubmissionForm onSubmit={onSubmit}/>
                                    <RecentSubmissionsTable 
                                        submissions={submissions} 
                                        contestId={contest.id} 
                                        userId={user?.id}
                                        problemId={problemId}
                                        wsUrl={wsUrl}
                                    />
                                </Stack>
                            </Paper>
                        </Stack>
                    </Box>
                </Box>
                </Box>
            </AppShellMain>
            <AppShellFooter withBorder={false}>
                <Footer/>
            </AppShellFooter>
        </Layout>
    );
};

export {Task};
