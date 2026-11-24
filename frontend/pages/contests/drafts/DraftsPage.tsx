import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import {api, unwrap} from "@/lib/api";

import {DraftsClient} from "./DraftsClient";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {slug, contestLogin} = routeParams as {slug: string; contestLogin: string};

  const [contestResponse, {data: me}] = await Promise.all([
    unwrap(
      api.GET("/organizations/{org_login}/contests/{contest_login}", {
        params: {path: {org_login: slug, contest_login: contestLogin}},
      }),
    ),
    api.GET("/users/me"),
  ]);

  if (!contestResponse?.contest) {
    throw {status: 404, message: "Контест не найден"};
  }

  const user = me?.user ?? null;
  if (!user?.id) {
    throw redirect(`/${slug}/${contestLogin}`);
  }

  const isManager = contestResponse.contest.permissions.manage;

  if (!contestResponse.contest.permissions.view_problems || !contestResponse.contest.enable_drafts) {
    throw redirect(`/${slug}/${contestLogin}`);
  }

  const isContestEnded = contestResponse.contest.end_time
    ? new Date(contestResponse.contest.end_time) <= new Date()
    : false;

  const {data: draftsResponse} = await api.GET(
    "/contests/{contest_id}/drafts",
    {
      params: {
        path: {contest_id: contestResponse.contest.id},
        query: {page: 1, pageSize: 50},
      },
    },
  );
  return {contestResponse, isManager, isContestEnded, draftsResponse};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {contestResponse, isManager, isContestEnded, draftsResponse} = data;
  const initialDrafts = draftsResponse?.drafts || [];
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}} pt="md">
      <title>Черновики</title>
      <DraftsClient
        contest={contestResponse.contest}
        isManager={isManager}
        isContestEnded={isContestEnded}
        initialDrafts={initialDrafts}
      />
    </Container>
  );
};

export default Page;
