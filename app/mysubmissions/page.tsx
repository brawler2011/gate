import {Metadata} from 'next';
import {getSubmissions, getContest} from '@/lib/actions';
import {Stack, Title, Container, Alert, ScrollArea} from '@mantine/core';
import {IconAlertCircle} from '@tabler/icons-react';
import {DefaultLayout} from '@/components/Layout';
import {NextPagination} from '@/components/Pagination';
import {SubmissionsListWithWS} from '@/components/SubmissionsList';
import {ContestHotbar} from '@/components/ContestHotbar';
import { getCurrentUser } from '@/lib/auth';
import { getMyContestRole } from '@/lib/contest-role';

export const metadata: Metadata = {
    title: 'Мои посылки',
    description: '',
};

interface SearchParams {
    page?: string;
    contestId?: string;
    userId?: string;
    problemId?: string;
    state?: string;
    order?: string;
    language?: string;
}

interface PageProps {
    searchParams: Promise<SearchParams>;
}

const PAGE_SIZE = 15;

const Page = async ({searchParams}: PageProps) => {
    const params = await searchParams;
    
    const parsedParams: {
        page: number;
        pageSize: number;
        contestId?: string;
        userId?: string;
        problemId?: string;
        state?: number;
        sortOrder?: 'asc' | 'desc';
        language?: number;
    } = {
        page: Number(params.page) || 1,
        pageSize: PAGE_SIZE,
    };
    
    if (params.contestId) parsedParams.contestId = params.contestId;
    if (params.userId) parsedParams.userId = params.userId;
    if (params.problemId) parsedParams.problemId = params.problemId;
    if (params.state) parsedParams.state = Number(params.state);
    if (params.order === 'asc' || params.order === 'desc') parsedParams.sortOrder = params.order;
    if (params.language) parsedParams.language = Number(params.language);
    
    console.log(parsedParams);
    const submissionsData = await getSubmissions(parsedParams);
    console.log(submissionsData);
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

    const queryParams: Record<string, string | number | undefined> = {
        page: parsedParams.page,
        pageSize: parsedParams.pageSize,
        contestId: parsedParams.contestId,
        userId: parsedParams.userId,
        problemId: parsedParams.problemId,
        state: parsedParams.state,
        order: parsedParams.sortOrder,
        language: parsedParams.language,
    };

    const token = 'access-token' in submissionsData ? submissionsData['access-token'] : undefined;
    const wsUrl = process.env.NEXT_PUBLIC_WS_core_URL! + "/submissions";

    // Load contest if contestId is provided
    let contestData = null;
    const user = await getCurrentUser();
    let contestRole = null;
    
    if (parsedParams.contestId) {
        contestData = await getContest(parsedParams.contestId);
        contestRole = user ? await getMyContestRole(parsedParams.contestId) : null;
    }

    return (
        <DefaultLayout>
            <Container size="lg" pt="md" pb="xl" px={{ base: 'xs', sm: 'md' }}>
                {contestData?.contest && (
                    <ContestHotbar 
                        contest={contestData.contest}
                        user={user}
                        contestRole={contestRole}
                        activeTab="mysubmissions"
                    />
                )}
                <Stack align="center" gap="md">
                    <Title>Мои посылки</Title>
                    <ScrollArea w="100%" type="auto">
                        <SubmissionsListWithWS
                            initialSubmissions={submissionsData}
                            wsUrl={wsUrl}
                            token={token || ''}
                            queryParams={queryParams}
                        />
                    </ScrollArea>
                    <NextPagination
                        pagination={submissionsData.pagination}
                        baseUrl="/mysubmissions"
                        queryParams={queryParams}
                    />
                </Stack>
            </Container>
        </DefaultLayout>
    );
};

export default Page;
