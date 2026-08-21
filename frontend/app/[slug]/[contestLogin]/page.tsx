import {
  Box,
  Center,
  Container,
  Stack,
  Text,
} from "@mantine/core";

import {ContestCountdown} from "@/components/contests/ContestCountdown";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import {ContestProblemsTable} from "./ContestProblemsTable";

import type {
  ContestModel,
  ContestProblemListItemModel,
} from "@/contracts/core/v1";
import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ slug: string; contestLogin: string }>;
};

export const generateMetadata = async ({
  params,
}: Props): Promise<Metadata> => {
  const {slug, contestLogin} = await params;

  const [error, response] = await api.getContest({orgLogin: slug, contestLogin});
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
  orgLogin: string;
  problems: Array<ContestProblemListItemModel>;
  isManager: boolean;
};

const Contest = ({
  contest,
  orgLogin,
  problems,
  isManager,
}: ContestProps) => {
  const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
  const showCountdown = !isManager && !hasStarted;

  return (
    <Container
      size="lg"
      pt={0}
      pb={{base: "md", sm: "lg", md: "xl"}}
    >
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
          contestLogin={contest.login}
          orgLogin={orgLogin}
          problems={problems}
          isManager={isManager}
        />
      )}
    </Container>
  );
};

const Page = async ({params}: Props): Promise<ReactNode> => {
  const {slug, contestLogin} = await params;
  const response = await unwrapAndCache(api.getContest)({orgLogin: slug, contestLogin});

  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(slug, contestLogin) : null;

  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );
  const isManager = checker.canManageContest(response.contest);

  return (
    <Contest
      contest={response.contest}
      orgLogin={slug}
      problems={response.problems || []}
      isManager={isManager}
    />
  );
};

export default Page;
