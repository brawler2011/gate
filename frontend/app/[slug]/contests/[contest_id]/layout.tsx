import {ContestHeaderNav} from "@/components/contests";
import {DefaultLayout} from "@/components/shared";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole} from "@/lib/permissions";

import type {ReactNode} from "react";

type Props = {
  children: ReactNode;
  params: Promise<{ slug: string; contest_id: string }>;
};

const ContestLayout = async ({
  children,
  params,
}: Props): Promise<ReactNode> => {
  const {slug, contest_id} = await params;
  const [response, orgData] = await Promise.all([
    unwrapAndCache(api.getContest)({contestId: contest_id}),
    unwrapAndCache(api.getOrganization)({login: slug}),
  ]);
  const [, me] = await api.getMe();
  const contestRole = me?.user ? await getMyContestRole(contest_id) : null;
  const org = orgData.organization;

  return (
    <DefaultLayout
      headerContest={{id: response.contest.id, title: response.contest.title}}
      headerOrganization={{id: org.id, login: org.login, name: org.name}}
      headerSecondaryNav={
        <ContestHeaderNav
          contest={response.contest}
          contestRole={contestRole}
          orgLogin={org.login}
        />
      }
    >
      {children}
    </DefaultLayout>
  );
};

export default ContestLayout;
