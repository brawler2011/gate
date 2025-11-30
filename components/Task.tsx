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
import type {SessionUser} from "@/lib/auth";
import type {ContestRole} from "@/lib/contest-role";
import { useRouter } from "next/navigation";

type PageProps = {
    tasks: ContestProblemListItemModel[]
    contest: ContestModel,
    task: ContestProblemModel,
    submissions: SubmissionsListItemModel[],
    problemId: string,
    contestId: string,
    user: SessionUser,
    contestRole: { role: ContestRole } | null,
    header: React.ReactNode
}

const Task = ({tasks, contest, task, submissions, problemId, contestId, user, contestRole, header}: PageProps) => {
    const router = useRouter();

    const onSubmit = async (
        submission: FormData,
        language: string
    ): Promise<number | null> => {
        const result = await submitSubmission(problemId, contestId, submission, language);
        
        if (result) {
            // Refresh the page data to update submissions list
            router.refresh();
        }
        
        return result;
    };

    return (
        <Layout>
            <AppShellHeader>
                {header}
            </AppShellHeader>
            <AppShellMain>
                <Box style={{ display: 'flex', gap: '32px', alignItems: 'flex-start' }}>
                    {/* Left Sidebar - скрыт на мобилках */}
                    <Box 
                        style={{ width: CONTEST_SIDEBAR_LEFT_WIDTH, marginLeft: '32px' }}
                        visibleFrom="sm"
                    >
                        <Paper 
                            shadow="sm" 
                            radius="md" 
                            p="md" 
                            withBorder 
                            bg="var(--mantine-color-gray-light)"
                        >
                            <Stack w={200}>
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
                                            c="var(--mantine-color-text)"
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
                            px={{ base: 'xs', sm: 'md' }}
                        >
                            <ContestHotbar 
                                contest={contest}
                                user={user}
                                contestRole={contestRole}
                            />
                            <Problem problem={task} letter={numberToLetters(task.position)}/>
                        </Container>
                    </Box>

                    {/* Right Sidebar - скрыт на мобилках */}
                    <Box 
                        style={{ width: CONTEST_SIDEBAR_RIGHT_WIDTH, marginRight: '32px' }}
                        visibleFrom="sm"
                    >
                        <Paper 
                            shadow="sm" 
                            radius="md" 
                            p="md" 
                            withBorder 
                            bg="var(--mantine-color-gray-light)"
                        >
                            <Stack>
                                <CreateSubmissionForm onSubmit={onSubmit}/>
                                <RecentSubmissionsTable submissions={submissions} contestId={contest.id} userId={user?.id}/>
                            </Stack>
                        </Paper>
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
