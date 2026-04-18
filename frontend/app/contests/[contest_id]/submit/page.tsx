import { ContestInfoPanel } from "@/components/contests/ContestInfoPanel";
import { Layout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { Footer } from "@/components/shared/Footer";
import { HeaderWithSession } from "@/components/shared/HeaderWithSession";
import { getContest } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { CONTEST_CONTENT_MAX_WIDTH } from "@/lib/constants";
import { buildContestHeaderNav } from "@/lib/contest-header-nav";
import { getMyContestRole } from "@/lib/contest-role";
import {
  AppShellFooter,
  AppShellHeader,
  AppShellMain,
  Box,
  Container,
} from "@mantine/core";
import { Metadata } from "next";
import classes from "../contestLayout.module.css";
import { SubmitSubmissionClient } from "./SubmitSubmissionClient";

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
  const contestHeaderNav = buildContestHeaderNav({
    contest: response!.contest,
    user,
    contestRole,
    activeTab: "submit",
  });

  return (
    <Layout>
      <AppShellHeader>
        <HeaderWithSession secondaryNavItems={contestHeaderNav} />
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
                style={{ maxWidth: "100%" }}
              >
                <SubmitSubmissionClient
                  contest={response!.contest}
                  problems={response!.problems || []}
                  user={user}
                />
              </Container>
            </Box>

            {/* Right Sidebar - Contest Info Panel - hidden on mobile */}
            <Box style={{ marginTop: "16px" }} visibleFrom="sm">
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
