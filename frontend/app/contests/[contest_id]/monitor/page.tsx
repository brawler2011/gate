import { ContestInfoPanel } from "@/components/contests/ContestInfoPanel";
import { DefaultLayout } from "@/components/shared";
import { getContest } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import {
  CONTEST_CONTENT_MAX_WIDTH,
  CONTEST_INFO_PANEL_COMPACT_WIDTH,
} from "@/lib/constants";
import { buildContestHeaderNav } from "@/lib/contest-header-nav";
import { getMyContestRole } from "@/lib/contest-role";
import { Box, Container, Text, Title } from "@mantine/core";
import { Metadata } from "next";
import classes from "../contestLayout.module.css";

const metadata: Metadata = {
  title: "Положение",
};

type PageProps = {
  params: Promise<{ contest_id: string }>;
};

const Page = async ({ params }: PageProps) => {
  const { contest_id } = await params;

  // Fetch contest data for the info panel
  const [, contestResponse] = await getContest(contest_id);
  const user = await getCurrentUser();
  const contestRole = user ? await getMyContestRole(contest_id) : null;
  const contestHeaderNav = contestResponse?.contest
    ? buildContestHeaderNav({
        contest: contestResponse.contest,
        user,
        contestRole,
        activeTab: "monitor",
      })
    : undefined;

  return (
    <DefaultLayout
      headerSecondaryNavItems={contestHeaderNav}
      headerOrganizationId={contestResponse?.contest.organization_id}
    >
      <Box className={classes.contestContainerWithLeftInfo}>
        {/* Left Sidebar - Contest Info Panel - hidden on mobile */}
        {contestResponse?.contest && (
          <Box
            style={{ width: CONTEST_INFO_PANEL_COMPACT_WIDTH }}
            visibleFrom="sm"
          >
            <ContestInfoPanel
              contest={contestResponse.contest}
              user={user}
              width={CONTEST_INFO_PANEL_COMPACT_WIDTH}
            />
          </Box>
        )}

        {/* Main Content */}
        <Box style={{ width: CONTEST_CONTENT_MAX_WIDTH }}>
          <Container
            size="xl"
            py="md"
            px={0}
            mx={0}
            style={{ maxWidth: "100%" }}
          >
            <Title order={2}>Монитор</Title>
            <Text c="dimmed" mt="md">
              Монитор скоро будет здесь!
            </Text>
          </Container>
        </Box>
      </Box>
    </DefaultLayout>
  );
};

export { Page as default, metadata };
