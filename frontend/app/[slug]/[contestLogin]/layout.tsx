import {Outlet, useParams} from "react-router-dom";

import {ContestHeaderNav, ContestEventsListener} from "@/components/contests";
import {DefaultLayout} from "@/components/shared";
import {useGetOrganization} from "@/contracts/core/v1/api";
import {useContestData} from "@/hooks/useContestData";
import {PageQueryState} from "@/lib/pageQuery";

import type {ReactNode} from "react";

const ContestLayout = (): ReactNode => {
  const {slug, contestLogin} = useParams() as { slug: string; contestLogin: string };
  const contest = useContestData(slug, contestLogin);
  const organization = useGetOrganization(slug);
  if (!contest.data) {
    return <PageQueryState query={contest} />;
  }
  if (!organization.data) {
    return <PageQueryState query={organization} />;
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
