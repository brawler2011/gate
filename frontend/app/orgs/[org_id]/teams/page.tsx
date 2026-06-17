import { OrgTeamsTab } from "@/components/orgs/OrgTeamsTab";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { listTeams, getOrganization } from "@/lib/actions";
import { buildOrgHeaderNav } from "@/lib/org-header-nav";
import { Container, Stack, Text, Title } from "@mantine/core";
import { notFound } from "next/navigation";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string }>;
};

export default async function OrgTeamsPage({ params, searchParams }: Props) {
  const { org_id } = await params;
  const { page } = await searchParams;
  const orgHeaderNav = buildOrgHeaderNav({ orgId: org_id, activeTab: "teams" });
  const currentPage = Number(page) > 0 ? Number(page) : 1;

  const [orgError, orgData] = await getOrganization(org_id);
  if (orgError) {
    if (orgError.status === 404) notFound();
    return (
      <DefaultLayout>
        <Container size="lg" py="lg">
          <ErrorDisplay error={orgError} />
        </Container>
      </DefaultLayout>
    );
  }

  const [teamsError, teamsData] = await listTeams(org_id, currentPage, 20);

  const org = orgData!.organization;
  const teams = teamsData?.teams ?? [];

  return (
    <DefaultLayout headerSecondaryNavItems={orgHeaderNav}>
      <Container size="lg" py="lg">
        <Stack gap="md">
          <div>
            <Title order={2}>{org.name}</Title>
            {org.description && (
              <Text c="dimmed" size="sm">
                {org.description}
              </Text>
            )}
          </div>

          {teamsError ? (
            <ErrorDisplay error={teamsError} />
          ) : (
            <OrgTeamsTab teams={teams} orgId={org_id} />
          )}
        </Stack>
      </Container>
    </DefaultLayout>
  );
}
