import {Container, Stack, Title} from "@mantine/core";
import {useLoaderData} from "react-router-dom";

import {DefaultLayout} from "@/components/shared";
import {SubmissionDetailsContent} from "@/components/submissions";
import {api, unwrap} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const solutionId = routeParams.submission_id!;
  const resp = await unwrap(
    api.GET("/submissions/{submission_id}", {
      params: {path: {submission_id: solutionId}},
    }),
  );

  const {submission} = resp;

  const {data: me} = await api.GET("/users/me");
  const user = me?.user ?? null;
  const contestRole =
    submission.organization_login && submission.contest_login
      ? await getMyContestRole(submission.organization_login, submission.contest_login)
      : null;
  const contestData =
    submission.organization_login && submission.contest_login
      ? await unwrap(
        api.GET("/organizations/{org_login}/contests/{contest_login}", {
          params: {
            path: {
              org_login: submission.organization_login,
              contest_login: submission.contest_login,
            },
          },
        }),
      )
      : null;
  return {submission, user, contestRole, contestData};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {submission, user, contestRole, contestData} = data;
  const checker = contestData?.contest
    ? new PermissionChecker(user, contestRole?.role ?? null, null, contestRole?.permissionsMask ?? null)
    : null;
  const canRejudge =
    contestData?.contest && checker ? checker.canRejudgeSubmissions(contestData.contest) : false;
  return (
    <DefaultLayout headerUser={user}>
      <title>{`Посылка #${submission.id.slice(0, 8)}`}</title>
      <Container size="lg" pt="md" pb="xl" px={{base: "xs", sm: "md"}}>
        <Stack gap="lg" align="flex-start" w="100%">
          <Title order={2}>Посылка #{submission.id.slice(0, 8)}...</Title>
          <SubmissionDetailsContent submission={submission} canRejudge={canRejudge} />
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export {Page as default};
