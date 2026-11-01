import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import {ContestMessagesClient} from "@/components/contests/messages/ContestMessagesClient";
import {api, unwrap} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

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
  const contestRole = user ? await getMyContestRole(slug, contestLogin) : null;

  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );

  if (!checker.canViewContest(contestResponse.contest)) {
    throw redirect(`/${slug}/${contestLogin}`);
  }

  const isModerator =
    contestRole?.role === "moderator" || contestRole?.role === "owner" || user?.role === "admin";

  return {slug, contestLogin, contestResponse, user, isModerator};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {slug, contestLogin, contestResponse, user, isModerator} =
    data;
  const problems = contestResponse.problems || [];
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}} pt="md">
      <title>Сообщения</title>
      <ContestMessagesClient
        contest={contestResponse.contest}
        orgLogin={slug}
        contestLogin={contestLogin}
        user={user}
        isModerator={isModerator}
        problems={problems}
      />
    </Container>
  );
};

export default Page;
