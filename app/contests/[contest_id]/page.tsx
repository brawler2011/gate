import { Footer } from "@/components/Footer";
import { HeaderWithSession } from "@/components/HeaderWithSession";
import { Layout } from "@/components/Layout";
import { ErrorDisplay } from "@/components/ErrorDisplay";
import { getContest } from "@/lib/actions";
import {
  AppShellFooter,
  AppShellHeader,
  AppShellMain,
  Box,
  Center,
  Container,
  Stack,
  Text,
} from "@mantine/core";
import { Metadata } from "next";
import type {
  ContestModel,
  ContestProblemListItemModel,
} from "../../../../contracts/core/v1";
import { ContestProblemsTable } from "./ContestProblemsTable";
import { ContestHotbar } from "@/components/ContestHotbar";
import { ContestInfoPanel } from "@/components/ContestInfoPanel";
import { getCurrentUser } from "@/lib/auth";
import { getMyContestRole } from "@/lib/contest-role";
import { CONTEST_CONTENT_MAX_WIDTH } from "@/lib/constants";
import classes from "./contestLayout.module.css";

type Props = {
  params: Promise<{ contest_id: string }>;
};

export const generateMetadata = async ({
  params,
}: Props): Promise<Metadata> => {
  const { contest_id } = await params;

  const [error, response] = await getContest(contest_id);
  if (error || !response) {
    return {
      title: "Ошибка загрузки контеста",
    };
  }

  return {
    title: response.contest.title,
    description: response.contest.title,
  };
};

type ContestProps = {
  contest: ContestModel;
  problems: Array<ContestProblemListItemModel>;
  user: Awaited<ReturnType<typeof getCurrentUser>>;
  contestRole: Awaited<ReturnType<typeof getMyContestRole>>;
};

const Contest = ({ contest, problems, user, contestRole }: ContestProps) => {
  return (
    <Layout>
      <AppShellHeader>
        <HeaderWithSession />
      </AppShellHeader>
      <AppShellMain>
        <Box maw="1920px" mx="auto" w="100%">
            <Box className={classes.contestContainer}>
            {/* Main Content */}
            <Box style={{ width: CONTEST_CONTENT_MAX_WIDTH }}>
              <Container
                size="lg"
                pt={0}
                pb={{ base: "md", sm: "lg", md: "xl" }}
                px={0}
                mx={0}
                style={{ maxWidth: '100%' }}
              >
                <ContestHotbar 
                  contest={contest} 
                  user={user}
                  contestRole={contestRole}
                  activeTab="tasks" 
                  align="left"
                >
                  {/* Tasks Section */}
                  {problems.length === 0 ? (
                    <Center py={{ base: "xl", md: "3xl" }}>
                      <Stack gap="md" align="center">
                        <Box component="div" style={{ fontSize: "2.5rem" }}>
                          📝
                        </Box>
                        <Text c="dimmed" size="md" fw={500}>
                          Нет задач в контесте
                        </Text>
                      </Stack>
                    </Center>
                  ) : (
                    <ContestProblemsTable contestId={contest.id} problems={problems} />
                  )}
                </ContestHotbar>
              </Container>
            </Box>

            {/* Right Sidebar - Contest Info Panel - hidden on mobile */}
            <Box 
              style={{ marginTop: '16px' }}
              visibleFrom="sm"
            >
              <ContestInfoPanel 
                contest={contest}
                user={user}
                contestRole={contestRole}
              />
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

const Page = async ({ params }: Props) => {
  const { contest_id } = await params;

  const [error, response] = await getContest(contest_id);
  if (error) return <ErrorDisplay error={error} />;

  // Get user and contest role for permissions
  const user = await getCurrentUser();
  const contestRole = user ? await getMyContestRole(contest_id) : null;

  return (
    <Contest 
      contest={response!.contest} 
      problems={response!.problems || []}
      user={user}
      contestRole={contestRole}
    />
  );
};

export default Page;
