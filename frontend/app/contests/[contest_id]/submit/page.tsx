import { Footer } from '@/components/shared/Footer';
import { HeaderWithSession } from '@/components/shared/HeaderWithSession';
import { Layout } from '@/components/shared';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { getContest } from "@/lib/actions";
import {
  AppShellFooter,
  AppShellHeader,
  AppShellMain,
  Box,
  Container,
} from "@mantine/core";
import { Metadata } from "next";
import { ContestHotbar } from '@/components/contests/ContestHotbar';
import { ContestInfoPanel } from '@/components/contests/ContestInfoPanel';
import { SubmitSubmissionClient } from "./SubmitSubmissionClient";
import { getCurrentUser } from "@/lib/auth";
import { getMyContestRole } from "@/lib/contest-role";
import { CONTEST_CONTENT_MAX_WIDTH } from "@/lib/constants";
import classes from "../contestLayout.module.css";

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

const Page = async ({ params }: Props) => {
  const { contest_id } = await params;

  const [error, response] = await getContest(contest_id);
  if (error) return <ErrorDisplay error={error} />;

  // Get user and contest role for permissions
  const user = await getCurrentUser();
  const contestRole = user ? await getMyContestRole(contest_id) : null;

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
                  contest={response!.contest}
                  user={user}
                  contestRole={contestRole}
                  activeTab="submit"
                  align="left"
                >
                  <SubmitSubmissionClient 
                    contest={response!.contest}
                    problems={response!.problems || []}
                    user={user}
                  />
                </ContestHotbar>
              </Container>
            </Box>

            {/* Right Sidebar - Contest Info Panel - hidden on mobile */}
            <Box 
              style={{ marginTop: '16px' }}
              visibleFrom="sm"
            >
              <ContestInfoPanel 
                contest={response!.contest}
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

export default Page;
