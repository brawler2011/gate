import {ContestHeaderNav} from "@/components/contests";
import {DefaultLayout} from "@/components/shared";
import {api, unwrapAndCache} from "@/lib/api";
import {getMyContestRole} from "@/lib/contest-role";

import type {ReactNode} from "react";

type Props = {
  children: ReactNode;
  params: Promise<{ contest_id: string }>;
};

const ContestLayout = async ({
  children,
  params,
}: Props): Promise<ReactNode> => {
  const {contest_id} = await params;
  const response = await unwrapAndCache(api.getContest)({contestId: contest_id});
  const [, me] = await api.getMe();
  const contestRole = me?.user ? await getMyContestRole(contest_id) : null;

  return (
    <DefaultLayout
      headerContest={{id: response.contest.id, title: response.contest.title}}
      headerOrganizationId={response.contest.organization_id}
      headerSecondaryNav={
        <ContestHeaderNav
          contest={response.contest}
          contestRole={contestRole}
        />
      }
    >
      {children}
    </DefaultLayout>
  );
};

export default ContestLayout;
