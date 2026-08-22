import {ContestHeaderNav, ContestEventsListener} from "@/components/contests";
import {DefaultLayout} from "@/components/shared";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole} from "@/lib/permissions";

import type {ReactNode} from "react";

type Props = {
  children: ReactNode;
  params: Promise<{ slug: string; contestLogin: string }>;
};

const ContestLayout = async ({
  children,
  params,
}: Props): Promise<ReactNode> => {
  const {slug, contestLogin} = await params;
  const [response, orgData] = await Promise.all([
    unwrapAndCache(api.getContest)({orgLogin: slug, contestLogin}),
    unwrapAndCache(api.getOrganization)({login: slug}),
  ]);
  const [, me] = await api.getMe();
  const contestRole = me?.user ? await getMyContestRole(slug, contestLogin) : null;
  const org = orgData.organization;

  const isModerator =
    contestRole?.role === "moderator" ||
    contestRole?.role === "owner" ||
    me?.user?.role === "admin";

  return (
    <DefaultLayout
      headerUser={me?.user ?? null}
      headerContest={{
        id: response.contest.id,
        login: response.contest.login,
        title: response.contest.title,
      }}
      headerOrganization={{id: org.id, login: org.login, name: org.name}}
      headerSecondaryNav={
        <ContestHeaderNav
          contest={response.contest}
          contestRole={contestRole}
          orgLogin={org.login}
          user={me?.user ?? null}
        />
      }
    >
      <ContestEventsListener
        contestId={response.contest.id}
        currentUserId={me?.user?.id}
        isModerator={isModerator}
      />
      {children}
    </DefaultLayout>
  );
};

export default ContestLayout;
