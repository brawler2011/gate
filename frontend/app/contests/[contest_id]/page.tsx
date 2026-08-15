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

import {ContestCountdown} from "@/components/contests/ContestCountdown";
import {Layout} from "@/components/shared";
import {Footer} from "@/components/shared/Footer";
import {HeaderWithSession} from "@/components/shared/HeaderWithSession";
import {api} from "@/lib/api";
import {unwrapAndCache} from "@/lib/api2";
import {buildContestHeaderNav} from "@/lib/contest-header-nav";
import {getMyContestRole} from "@/lib/contest-role";
import {PermissionChecker} from "@/lib/permissions";

import {ContestProblemsTable} from "./ContestProblemsTable";

import type {
  ContestModel,
  ContestProblemListItemModel,
  UserModel,
} from "@/contracts/core/v1";
import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ contest_id: string }>;
};

export const generateMetadata = async ({
  params,
}: Props): Promise<Metadata> => {
  const {contest_id} = await params;

  const [error, response] = await api.getContest({contestId: contest_id});
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
  user: UserModel | null;
  contestHeaderNav: ReturnType<typeof buildContestHeaderNav>;
  isManager: boolean;
};

const Contest = ({
  contest,
  problems,
  user: _user,
  contestHeaderNav,
  isManager,
}: ContestProps) => {
  const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
  const showCountdown = !isManager && !hasStarted;
  return (
    <Layout>
      <AppShellHeader>
        <HeaderWithSession
          secondaryNavItems={contestHeaderNav}
          organizationId={contest.organization_id}
          contest={{id: contest.id, title: contest.title}}
        />
      </AppShellHeader>
      <AppShellMain>
        <Container
          size="lg"
          pt={0}
          pb={{base: "md", sm: "lg", md: "xl"}}
        >
          {/* Tasks Section */}
          {showCountdown && (
            <ContestCountdown
              startTime={contest.start_time!}
              title={contest.title}
            />
          )}
          {!showCountdown && problems.length === 0 && (
            <Center py={{base: "xl", md: "3xl"}}>
              <Stack gap="md" align="center">
                <Box component="div" style={{fontSize: "2.5rem"}}>
                  📝
                </Box>
                <Text c="dimmed" size="md" fw={500}>
                  Нет задач в контесте
                </Text>
              </Stack>
            </Center>
          )}
          {!showCountdown && problems.length > 0 && (
            <ContestProblemsTable
              contestId={contest.id}
              problems={problems}
            />
          )}
        </Container>
      </AppShellMain>

      <AppShellFooter withBorder={false}>
        <Footer />
      </AppShellFooter>
    </Layout>
  );
};

const Page = async ({params}: Props): Promise<ReactNode> => {
  const {contest_id} = await params;
  const response = await unwrapAndCache(api.getContest)({contestId: contest_id});

  // Get user and contest role for permissions
  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(contest_id) : null;
  const contestHeaderNav = buildContestHeaderNav({
    contest: response.contest,
    user,
    contestRole,
    activeTab: "tasks",
  });

  const checker = new PermissionChecker(user, contestRole?.role ?? null, null, contestRole?.permissionsMask ?? null);
  const isManager = checker.canManageContest(response.contest);

  return (
    <Contest
      contest={response.contest}
      problems={response.problems || []}
      user={user}
      contestHeaderNav={contestHeaderNav}
      isManager={isManager}
    />
  );
};

export default Page;
