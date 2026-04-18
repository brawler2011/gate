import { OrgContestsTab } from "@/components/orgs/OrgContestsTab";
import { OrgMembersTab } from "@/components/orgs/OrgMembersTab";
import { OrgProblemsTab } from "@/components/orgs/OrgProblemsTab";
import { OrgTeamsTab } from "@/components/orgs/OrgTeamsTab";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import type { ApiError } from "@/lib/api";
import type { OrgOverviewTab } from "@/lib/org-header-nav";
import type {
  ContestModel,
  OrganizationMemberModel,
  ProblemsListItemModel,
  TeamModel,
} from "@contracts/gateway/v1";

export type OrgTabsActiveTab = OrgOverviewTab;

type Props = {
  members: OrganizationMemberModel[];
  teams: TeamModel[];
  problems: ProblemsListItemModel[];
  contests: ContestModel[];
  orgId: string;
  activeTab: OrgTabsActiveTab;
  membersError: ApiError | null;
  teamsError: ApiError | null;
  problemsError: ApiError | null;
  contestsError: ApiError | null;
};

export function OrgTabs({
  members,
  teams,
  problems,
  contests,
  orgId,
  activeTab,
  membersError,
  teamsError,
  problemsError,
  contestsError,
}: Props) {
  if (activeTab === "problems") {
    return problemsError ? (
      <ErrorDisplay error={problemsError} />
    ) : (
      <OrgProblemsTab problems={problems} />
    );
  }

  if (activeTab === "teams") {
    return teamsError ? (
      <ErrorDisplay error={teamsError} />
    ) : (
      <OrgTeamsTab teams={teams} orgId={orgId} />
    );
  }

  if (activeTab === "members") {
    return membersError ? (
      <ErrorDisplay error={membersError} />
    ) : (
      <OrgMembersTab members={members} />
    );
  }

  return contestsError ? (
    <ErrorDisplay error={contestsError} />
  ) : (
    <OrgContestsTab contests={contests} />
  );
}
