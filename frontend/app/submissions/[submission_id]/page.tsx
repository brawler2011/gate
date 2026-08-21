import {
  Group,
  Stack,
  Table,
  TableTbody,
  TableTd,
  TableTh,
  TableThead,
  TableTr,
  Text,
  Title,
  ScrollArea,
  Container,
} from "@mantine/core";
import Link from "next/link";

import {DefaultLayout} from '@/components/shared';
import {CodeBlock} from '@/components/shared/CodeBlock';
import {ErrorDisplay} from '@/components/shared/ErrorDisplay';
import {SingleSubmissionRejudgeButton} from "@/components/submissions";
import {api, unwrapAndCache} from "@/lib/api";
import {
  LangNameToString,
  LangString,
  ProblemTitle,
  StateColor,
  StateString,
  TimeBeautify,
} from "@/lib/lib";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ submission_id: string }>;
};

const metadata: Metadata = {
  title: `Просмотр посылки`,
  description: "",
};

const Page = async (props: Props): Promise<ReactNode> => {
  const solutionId = (await props.params).submission_id;
  const [error, resp] = await api.getSubmission({submissionId: solutionId});

  if (error) {
    return <ErrorDisplay error={error} />;
  }
  if (!resp) {
    return <ErrorDisplay error={{status: 404, message: "Посылка не найдена"}} />;
  }

  const {submission} = resp;

  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = submission.contest_id ? await getMyContestRole(submission.contest_id) : null;
  const contestData = submission.contest_id
    ? await unwrapAndCache(api.getContest)({contestId: submission.contest_id})
    : null;

  const checker = contestData?.contest
    ? new PermissionChecker(user, contestRole?.role ?? null, null, contestRole?.permissionsMask ?? null)
    : null;

  const canRejudge = contestData?.contest && checker ? checker.canRejudgeSubmissions(contestData.contest) : false;

  const rows = [submission].map((submission) => (
    <TableTr key={submission.id}>
      <TableTd ta="center">
        <Text>{TimeBeautify(submission.created_at)}</Text>
      </TableTd>
      <TableTd ta="center">
        <Link href={`/users/${submission.user_id}`} style={{color: 'inherit'}}>
          <Text span td="underline">
            {submission.username}
          </Text>
        </Link>
      </TableTd>
      <TableTd ta="center">
        <Link href={`/contests/${submission.contest_id}/problems/${submission.problem_id}`} style={{color: 'inherit'}}>
          <Text span td="underline">
            {ProblemTitle(submission.position, submission.problem_title)}
          </Text>
        </Link>
      </TableTd>
      <TableTd ta="center">
        <Text>{LangString(submission.language)}</Text>
      </TableTd>
      <TableTd ta="center">
        <Text c={StateColor(submission.state)} fw={500}>
          {StateString(submission.state)}
        </Text>
      </TableTd>
      <TableTd ta="center">
        <Text>{submission.time_stat} ms</Text>
      </TableTd>
      <TableTd ta="center">
        <Text>{submission.memory_stat} KB</Text>
      </TableTd>
    </TableTr>
  ));

  return (
    <DefaultLayout>
      <Container size="lg" pt="md" pb="xl" px={{base: 'xs', sm: 'md'}}>
        <Stack align="center" gap="md">
          <ScrollArea w="100%" type="auto">
            <Table horizontalSpacing="sm" style={{minWidth: 700}}>
              <TableThead>
                <TableTr>
                  <TableTh ta="center">Когда</TableTh>
                  <TableTh ta="center">Кто</TableTh>
                  <TableTh ta="center">Задача</TableTh>
                  <TableTh ta="center">Язык</TableTh>
                  <TableTh ta="center">Вердикт</TableTh>
                  <TableTh ta="center">Время</TableTh>
                  <TableTh ta="center">Память</TableTh>
                </TableTr>
              </TableThead>
              <TableTbody>{rows}</TableTbody>
            </Table>
          </ScrollArea>
          <Stack align="flex-start" w="100%">
            <Group justify="space-between" align="center" w="100%">
              <Title order={2}>Код решения</Title>
              {canRejudge && submission.contest_id && (
                <SingleSubmissionRejudgeButton
                  contestId={submission.contest_id}
                  submissionId={submission.id}
                />
              )}
            </Group>
            <CodeBlock
              code={submission.submission}
              language={LangNameToString(submission.language)}
            />
          </Stack>
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export {Page as default, metadata};
