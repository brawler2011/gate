import {Container} from "@mantine/core";
import {redirect} from "next/navigation";

import {ContestMessagesClient} from "@/components/contests/messages/ContestMessagesClient";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {ContestClarificationModel} from "@/contracts/core/v1";
import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Сообщения контеста",
  description: "Объявления от жюри и вопросы по условиям задач",
};

type PageProps = {
  params: Promise<{ slug: string; contestLogin: string }>;
};

const Page = async ({params}: PageProps): Promise<ReactNode> => {
  const {slug, contestLogin} = await params;

  const [contestResponse, [, me]] = await Promise.all([
    unwrapAndCache(api.getContest)({orgLogin: slug, contestLogin}),
    api.getMe(),
  ]);

  if (!contestResponse?.contest) {
    return <ErrorDisplay error={{status: 404, message: "Контест не найден"}} />;
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
    contestRole?.role === "moderator" ||
    contestRole?.role === "owner" ||
    user?.role === "admin";

  const [, announcementsRes] = await api.listContestAnnouncements({
    orgLogin: slug,
    contestLogin,
    page: 1,
    pageSize: 100,
  });

  let initialClarifications: ContestClarificationModel[] = [];
  if (user) {
    const [, clarificationsRes] = await api.listContestClarifications({
      orgLogin: slug,
      contestLogin,
      page: 1,
      pageSize: 100,
    });
    initialClarifications = clarificationsRes?.clarifications || [];
  }

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
