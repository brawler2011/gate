import {Outlet, useParams} from "react-router-dom";

import {ContestEventsListener} from "@/components/contests/ContestEventsListener";
import {ContestHeaderNav} from "@/components/contests/ContestHeaderNav";
import {DefaultLayout} from "@/components/shared/DefaultLayout";
import {QueryState} from "@/components/shared/QueryState";
import {useContestData} from "@/hooks/useContestData";
import {api} from "@/lib/api";

import type {ReactNode} from "react";

const ContestLayout = (): ReactNode => {
  const {slug, contestLogin} = useParams() as { slug: string; contestLogin: string };
  const contest = useContestData(slug, contestLogin);
  const organization = api.useQuery("get", "/organizations/{login}", {
    params: {path: {login: slug}},
  });
  if (!contest.data) {
    return <QueryState error={contest.error} />;
  }
  if (!organization.data) {
    return <QueryState error={organization.error} />;
  }
  const {response, user, contestRole} = contest.data;
  const org = organization.data.organization;
  const isModerator =
    contestRole?.role === "moderator" || contestRole?.role === "owner" || user?.role === "admin";
  return (
    <DefaultLayout
      headerUser={user}
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
          user={user}
        />
      }
    >
      <ContestEventsListener
        contestId={response.contest.id}
        currentUserId={user?.id}
        isModerator={isModerator}
      />
      <Outlet />
    </DefaultLayout>
  );
};
export default ContestLayout;
