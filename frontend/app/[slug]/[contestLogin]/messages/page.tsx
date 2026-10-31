import {Container} from "@mantine/core";

import {ContestMessagesClient} from "@/components/contests/messages/ContestMessagesClient";
import {api, unwrap} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";
import {redirect} from "@/lib/routeErrors";

import type {ContestClarificationModel} from "@/contracts/core/v1/models";
import type {ReactNode} from "react";

const loadPage = async ({params}: PageProps) => {
  const {slug, contestLogin} = params;

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
    redirect(`/${slug}/${contestLogin}`);
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
  return {
    slug,
    contestLogin,
    contestResponse,
    me,
    user,
    contestRole,
    checker,
    isModerator,
    announcementsRes,
    initialClarifications,
  };
};

type PageProps = {
  params: { slug: string; contestLogin: string };
};

const Page = ({params}: PageProps): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/[contestLogin]/messages/page.tsx", params], () =>
    loadPage({params}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {slug, contestLogin, contestResponse, user, isModerator, announcementsRes, initialClarifications} =
    routeQuery.data;
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
