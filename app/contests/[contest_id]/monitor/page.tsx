import {
    Box,
    Container,
    Text,
    Title
} from '@mantine/core';
import {Metadata} from "next";
import {DefaultLayout} from '@/components/shared';
import {ContestInfoPanel} from '@/components/contests/ContestInfoPanel';
import {getContest} from "@/lib/actions";
import {getCurrentUser} from "@/lib/auth";
import {getMyContestRole} from "@/lib/contest-role";
import { CONTEST_CONTENT_MAX_WIDTH } from "@/lib/constants";
import classes from '../contestLayout.module.css';

const metadata: Metadata = {
    title: "Положение"
}

type PageProps = {
    params: Promise<{ contest_id: string }>
}

const Page = async ({params}: PageProps) => {
    const { contest_id } = await params;
    
    // Fetch contest data for the info panel
    const [, contestResponse] = await getContest(contest_id);
    const user = await getCurrentUser();
    const contestRole = user ? await getMyContestRole(contest_id) : null;

    return (
        <DefaultLayout>
                <Box className={classes.contestContainer}>
                    {/* Main Content */}
                    <Box style={{ width: CONTEST_CONTENT_MAX_WIDTH }}>
                        <Container size="xl" py="md" px={0} mx={0} style={{ maxWidth: '100%' }}>
                            <Title order={2}>Положение</Title>
                            <Text c="dimmed" mt="md">
                                Функция мониторинга находится в разработке.
                            </Text>
                        </Container>
                    </Box>

                    {/* Right Sidebar - Contest Info Panel - hidden on mobile */}
                    {contestResponse?.contest && (
                        <Box 
                            style={{ marginTop: '16px' }}
                            visibleFrom="sm"
                        >
                            <ContestInfoPanel 
                                contest={contestResponse.contest}
                                user={user}
                                contestRole={contestRole}
                            />
                        </Box>
                    )}
                </Box>
        </DefaultLayout>
    );
};

export {Page as default, metadata};
