import {Metadata} from 'next';
import {getSubmissions, getContest} from '@/lib/actions';
import {Stack, Container, Alert, Paper, Group, Box, Center} from '@mantine/core';
import {IconAlertCircle} from '@tabler/icons-react';
import {DefaultLayout} from '@/components/Layout';
import {NextPagination} from '@/components/Pagination';
import {SubmissionsListClient} from '@/components/SubmissionsList';
import {ContestHotbar} from '@/components/ContestHotbar';
import {ContestInfoPanel} from '@/components/ContestInfoPanel';
import {ErrorDisplay} from '@/components/ErrorDisplay';
import { getCurrentUser } from '@/lib/auth';
import { getMyContestRole } from '@/lib/contest-role';
import { CONTEST_CONTENT_MAX_WIDTH } from '@/lib/constants';
import classes from '../contestLayout.module.css';

export const metadata: Metadata = {
    title: 'Мои посылки',
    description: '',
};

interface SearchParams {
    page?: string;
    userId?: string;
    problemId?: string;
    state?: string;
    order?: string;
    language?: string;
}

interface PageProps {
    params: Promise<{ contest_id: string }>;
    searchParams: Promise<SearchParams>;
}

const PAGE_SIZE = 20;

const Page = async ({params, searchParams}: PageProps) => {
    const { contest_id } = await params;
    const queryParams = await searchParams;
    
    const parsedParams: {
        page: number;
        pageSize: number;
        contestId: string;
        userId?: string;
        problemId?: string;
        state?: number;
        sortOrder?: 'asc' | 'desc';
        language?: number;
    } = {
        page: Number(queryParams.page) || 1,
        pageSize: PAGE_SIZE,
        contestId: contest_id,
    };
    
    if (queryParams.userId) parsedParams.userId = queryParams.userId;
    if (queryParams.problemId) parsedParams.problemId = queryParams.problemId;
    if (queryParams.state) parsedParams.state = Number(queryParams.state);
    if (queryParams.order === 'asc' || queryParams.order === 'desc') parsedParams.sortOrder = queryParams.order;
    if (queryParams.language) parsedParams.language = Number(queryParams.language);
    
    const [error, submissionsData] = await getSubmissions(parsedParams);
    
    if (error) return <ErrorDisplay error={error} />;
    
    if (!submissionsData) {
        return (
            <DefaultLayout>
                <Container size="lg" py="xl">
                    <Alert 
                        icon={<IconAlertCircle size="1rem" />} 
                        title="Ошибка загрузки" 
                        color="red"
                    >
                        Не удалось загрузить список решений. Попробуйте обновить страницу.
                    </Alert>
                </Container>
            </DefaultLayout>
        );
    }

    const nextQueryParams: Record<string, string | number | undefined> = {
        page: parsedParams.page,
        pageSize: parsedParams.pageSize,
        userId: parsedParams.userId,
        problemId: parsedParams.problemId,
        state: parsedParams.state,
        order: parsedParams.sortOrder,
        language: parsedParams.language,
    };

    // Remove trailing slash if present to avoid double slashes
    const wsBaseUrl = (process.env.NEXT_PUBLIC_WS_core_URL || '').replace(/\/+$/, '');

    const [contestError, contestResponse] = await getContest(contest_id);
    const contestData = contestResponse;
    
    const user = await getCurrentUser();
    const contestRole = user ? await getMyContestRole(contest_id) : null;

    return (
        <DefaultLayout>
                <Box className={classes.contestContainer}>
                    {/* Main Content */}
                    <Box style={{ width: CONTEST_CONTENT_MAX_WIDTH, minWidth: 0, overflow: 'hidden' }}>
                        <Container size="xl" pt={0} pb="xl" px={0} mx={0} style={{ maxWidth: '100%' }}>
                            {contestData?.contest ? (
                                <ContestHotbar 
                                    contest={contestData.contest}
                                    user={user!}
                                    contestRole={contestRole}
                                    activeTab="mysubmissions"
                                    maxWidth="100%"
                                    align="left"
                                >
                                    <Paper 
                                        withBorder 
                                        p="md" 
                                        w="100%" 
                                        shadow="sm" 
                                        radius="md"
                                        style={{ 
                                            backgroundColor: 'light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))',
                                            borderColor: 'light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))'
                                        }}
                                    >
                                        <Stack gap="md">
                                            <SubmissionsListClient
                                                initialSubmissions={submissionsData.submissions}
                                                wsUrl={wsBaseUrl + "/submissions"}
                                                filter={{
                                                    contestId: contest_id,
                                                    userId: parsedParams.userId,
                                                    problemId: parsedParams.problemId,
                                                }}
                                                pageSize={PAGE_SIZE}
                                                page={parsedParams.page}
                                                sortOrder={parsedParams.sortOrder}
                                            />
                                            <Group justify="center">
                                                <NextPagination
                                                    pagination={submissionsData.pagination}
                                                    baseUrl={`/contests/${contest_id}/mysubmissions`}
                                                    queryParams={nextQueryParams}
                                                />
                                            </Group>
                                        </Stack>
                                    </Paper>
                                </ContestHotbar>
                            ) : (
                                <ErrorDisplay error={contestError || {status: 404, message: "Contest not found"}} />
                            )}
                        </Container>
                    </Box>

                    {/* Right Sidebar - Contest Info Panel - hidden on mobile */}
                    {contestData?.contest && (
                        <Box 
                            style={{ marginTop: '16px' }}
                            visibleFrom="sm"
                        >
                            <ContestInfoPanel 
                                contest={contestData.contest}
                                user={user}
                                contestRole={contestRole}
                            />
                        </Box>
                    )}
                </Box>
        </DefaultLayout>
    );
};

export default Page;

