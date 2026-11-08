import {Container} from "@mantine/core";
import {useLoaderData, Navigate} from "react-router-dom";

import {ContestMonitorTable} from "@/components/contests";
import {api, unwrap} from "@/lib/api";
import {PermissionChecker} from "@/lib/permissions";

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
  if (contestResponse?.contest) {
    const checker = new PermissionChecker();
    isManager = checker.canManageContest(contestResponse.contest);

    if (!checker.canViewMonitor(contestResponse.contest)) {
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
      />
    </Container>
  );
};

export default Page;
