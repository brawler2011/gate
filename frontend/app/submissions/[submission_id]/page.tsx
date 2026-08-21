import {
  Container,
  Stack,
  Title,
} from "@mantine/core";

import {DefaultLayout} from '@/components/shared';
import {ErrorDisplay} from '@/components/shared/ErrorDisplay';
import {SubmissionDetailsContent} from "@/components/submissions";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ submission_id: string }>;
};

const metadata: Metadata = {
  title: `Просмотр посылки`,
  description: "Детали посылки и протокол тестирования",
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
  const contestRole =
    submission.organization_login && submission.contest_login
      ? await getMyContestRole(submission.organization_login, submission.contest_login)
      : null;
  const contestData =
    submission.organization_login && submission.contest_login
      ? await unwrapAndCache(api.getContest)({
        orgLogin: submission.organization_login,
        contestLogin: submission.contest_login,
      })
      : null;

  const checker = contestData?.contest
    ? new PermissionChecker(user, contestRole?.role ?? null, null, contestRole?.permissionsMask ?? null)
    : null;

  const canRejudge = contestData?.contest && checker ? checker.canRejudgeSubmissions(contestData.contest) : false;

  return (
    <DefaultLayout headerUser={user}>
      <Container size="lg" pt="md" pb="xl" px={{base: 'xs', sm: 'md'}}>
        <Stack gap="lg" align="flex-start" w="100%">
          <Title order={2}>
            Посылка #{submission.id.slice(0, 8)}...
          </Title>
          <SubmissionDetailsContent
            submission={submission}
            canRejudge={canRejudge}
          />
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export {Page as default, metadata};
