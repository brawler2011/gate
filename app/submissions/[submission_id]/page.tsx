import { CodeBlock } from "@/components/CodeBlock";
import { DefaultLayout } from "@/components/Layout";
import { ErrorDisplay } from "@/components/ErrorDisplay";
import { getSubmission } from "@/lib/actions";
import {
  LangNameToString,
  LangString,
  ProblemTitle,
  StateColor,
  StateString,
  TimeBeautify,
} from "@/lib/lib";
import {
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
import { Metadata } from "next";
import Link from "next/link";

type Props = {
  params: Promise<{ submission_id: string }>;
};

const metadata: Metadata = {
  title: `Просмотр посылки`,
  description: "",
};

const Page = async (props: Props) => {
  const solutionId = (await props.params).submission_id;
  const [error, resp] = await getSubmission(solutionId);

  if (error) return <ErrorDisplay error={error} />;
  if (!resp) return <ErrorDisplay error={{ status: 404, message: "Посылка не найдена" }} />;

  const { submission } = resp;

  const rows = [submission].map((submission) => (
    <TableTr key={submission.id}>
      <TableTd ta="center">
        <Text>{TimeBeautify(submission.created_at)}</Text>
      </TableTd>
      <TableTd ta="center">
        <Text
          component={Link}
          href={`/users/${submission.user_id}`}
          td="underline"
        >
          {submission.username}
        </Text>
      </TableTd>
      <TableTd ta="center">
        <Text
          component={Link}
          href={`/contests/${submission.contest_id}/problems/${submission.problem_id}`}
          td="underline"
        >
          {ProblemTitle(submission.position, submission.problem_title)}
        </Text>
      </TableTd>
      <TableTd ta="center">
        <Text>{LangString(submission.language)}</Text>
      </TableTd>
      <TableTd ta="center">
        <Text c={StateColor(submission.state)} fw={500}>
          {StateString(submission.state, submission.failed_test)}
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
      <Container size="lg" pt="md" pb="xl" px={{ base: 'xs', sm: 'md' }}>
        <Stack align="center" gap="md">
          <ScrollArea w="100%" type="auto">
            <Table horizontalSpacing="sm" style={{ minWidth: 700 }}>
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
            <Title order={2}>Код решения</Title>
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

export { Page as default, metadata };
