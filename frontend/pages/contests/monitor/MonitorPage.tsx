import {Container} from "@mantine/core";
import {useLoaderData, Navigate} from "react-router-dom";

import {ContestMonitorTable} from "@/components/contests";
import {api, unwrap} from "@/lib/api";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {slug, contestLogin} = routeParams as {slug: string; contestLogin: string};

  const [contestResponse] = await Promise.all([
    unwrap(
      api.GET("/organizations/{org_login}/contests/{contest_login}", {
        params: {path: {org_login: slug, contest_login: contestLogin}},
      }),
    ),
  ]);
  return {slug, contestLogin, contestResponse};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {slug, contestLogin, contestResponse} = data;
  let isManager = false;
  let allowVirtual = false;
  if (contestResponse?.contest) {
    allowVirtual = contestResponse.contest.allow_virtual;
    isManager = contestResponse.contest.permissions.manage;

    if (!contestResponse.contest.permissions.view_scoreboard) {
      return <Navigate to={`/${slug}/${contestLogin}`} replace />;
    }
  }
  return (
    <Container size="lg" py="md">
      <title>Монитор</title>
      <ContestMonitorTable
        orgLogin={slug}
        contestLogin={contestLogin}
        isManager={isManager}
        allowVirtual={allowVirtual}
      />
    </Container>
  );
};

export default Page;
