import {Box, Center, Container, Stack, Text} from "@mantine/core";
import {useParams} from "react-router-dom";

import {ContestCountdown} from "@/components/contests/ContestCountdown";
import {ContestJoinButton} from "@/components/contests/ContestJoinButton";
import {QueryState} from "@/components/shared/QueryState";
import {useContestData} from "@/hooks/useContestData";
import {PermissionChecker} from "@/lib/permissions";

import {ContestProblemsTable} from "./ContestProblemsTable";

import type {ContestModel, ContestProblemListItemModel} from "@/contracts/core/v1/models";
import type {ReactNode} from "react";

type ContestProps = {
  contest: ContestModel;
  orgLogin: string;
  problems: Array<ContestProblemListItemModel>;
  isManager: boolean;
  isAuthenticated: boolean;
  hasAccess: boolean;
};

const Contest = ({contest, orgLogin, problems, isManager, isAuthenticated, hasAccess}: ContestProps) => {
  const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
  const showCountdown = !isManager && !hasStarted;

  return (
    <Container size="lg" pt={0} pb={{base: "md", sm: "lg", md: "xl"}}>
      <ContestJoinButton
        contest={contest}
        orgLogin={orgLogin}
        isAuthenticated={isAuthenticated}
        hasAccess={hasAccess || isManager}
      />
      {showCountdown && <ContestCountdown startTime={contest.start_time!} title={contest.title} />}
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
          contestLogin={contest.login}
          orgLogin={orgLogin}
          problems={problems}
          isManager={isManager}
          hideStatements={contest.hide_statements}
        />
      )}
    </Container>
  );
};

const Page = (): ReactNode => {
  const params = useParams() as {slug: string; contestLogin: string};
  const {slug, contestLogin} = params;
  const routeQuery = useContestData(slug, contestLogin);
  if (!routeQuery.data) {
    return <QueryState error={routeQuery.error} />;
  }
  const {response, user, contestRole} = routeQuery.data;
  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );
  const isManager = checker.canManageContest(response.contest);
  const hasAccess = Boolean(contestRole?.role);
  return (
    <Contest
      contest={response.contest}
      orgLogin={slug}
      problems={response.problems || []}
      isManager={isManager}
      isAuthenticated={Boolean(user)}
      hasAccess={hasAccess}
    />
  );
};

export default Page;
