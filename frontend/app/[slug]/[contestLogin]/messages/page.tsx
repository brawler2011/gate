import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import {ContestMessagesClient} from "@/components/contests/messages/ContestMessagesClient";
import {api, unwrap} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {ContestClarificationModel} from "@/contracts/core/v1/models";
import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {slug, contestLogin} = routeParams as {slug: string; contestLogin: string};

  const [contestResponse, [, me]] = await Promise.all([
    unwrap(api.getContest)(slug, contestLogin),
    api.getMe(),
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

  const [, announcementsRes] = await api.listContestAnnouncements(slug, contestLogin, {
    page: 1, pageSize: 100
  });

  let initialClarifications: ContestClarificationModel[] = [];
  if (user) {
    const [, clarificationsRes] = await api.listContestClarifications(slug, contestLogin, {
      page: 1, pageSize: 100
    });
    initialClarifications = clarificationsRes?.clarifications || [];
  }
  return {slug, contestLogin, contestResponse, user, isModerator, announcementsRes, initialClarifications};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {slug, contestLogin, contestResponse, user, isModerator, announcementsRes, initialClarifications} =
    data;
  const initialAnnouncements = announcementsRes?.announcements || [];
  const problems = contestResponse.problems || [];
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}} pt="md">
      <ContestMessagesClient
        contest={contestResponse.contest}
        orgLogin={slug}
        contestLogin={contestLogin}
        user={user}
        isModerator={isModerator}
        problems={problems}
        initialAnnouncements={initialAnnouncements}
        initialClarifications={initialClarifications}
      />
    </Container>
  );
};

export default Page;
