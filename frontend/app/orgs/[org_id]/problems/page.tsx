import { OrgProblemsTab } from "@/components/orgs/OrgProblemsTab";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { getProblems, getOrganization } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { buildOrgHeaderNav } from "@/lib/org-header-nav";
import { Container, Stack, Text, Title } from "@mantine/core";
import { notFound } from "next/navigation";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string }>;
};

export default async function OrgProblemsPage({ params, searchParams }: Props) {
  const { org_id } = await params;
  const { page } = await searchParams;
  const orgHeaderNav = buildOrgHeaderNav({ orgId: org_id, activeTab: "problems" });
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

  const [
    [problemsError, problemsData],
    currentUser,
  ] = await Promise.all([
    getProblems(currentPage, 20, undefined, undefined, undefined, org_id),
    getCurrentUser(),
  ]);

  const org = orgData!.organization;
  const problems = problemsData?.problems ?? [];
  const problemsPagination = problemsData?.pagination ?? { page: 1, total: 1 };
  const isAuthenticated = currentUser !== null;

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

          {problemsError ? (
            <ErrorDisplay error={problemsError} />
          ) : (
            <OrgProblemsTab
              problems={problems}
              pagination={problemsPagination}
              org={org}
              isAuthenticated={isAuthenticated}
            />
          )}
        </Stack>
      </Container>
    </DefaultLayout>
  );
}
