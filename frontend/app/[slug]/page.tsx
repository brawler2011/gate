import {Container} from "@mantine/core";
import {notFound, redirect} from "next/navigation";
import {Suspense} from "react";

import {OrgContestsTab} from "@/components/orgs/OrgContestsTab";
import {DefaultLayout} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {
  ProfileContainer,
  ProfileHeader,
  UserContestsSection,
  UserContestsSkeleton,
} from "@/components/users";
import {api, unwrapAndCache} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ contestsPage?: string; page?: string; search?: string }>;
};

const getUser = unwrapAndCache(api.getUser);
const getOrganization = unwrapAndCache(api.getOrganization);

export const generateMetadata = async ({params}: Props): Promise<Metadata> => {
  const {slug} = await params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  if (decoded.startsWith("@")) {
    const cleanUsername = decoded.slice(1);
    if (!cleanUsername) {
      notFound();
    }
    const data = await getUser({username: cleanUsername});
    return {title: `${data.user.username}`};
  }

  const [orgError, orgData] = await api.getOrganization({login: decoded});
  if (orgError || !orgData) {
    return {title: "Организация"};
  }

  return {title: orgData.organization.name};
};

const Page = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {slug} = await params;
  const {contestsPage, page, search} = await searchParams;

  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  // Handle User profile (@username)
  if (decoded.startsWith("@")) {
    const cleanUsername = decoded.slice(1);
    if (!cleanUsername) {
      notFound();
    }

    const userPage = parsePage(contestsPage);
    if (!userPage) {
      redirect(`/@${cleanUsername}`);
    }

    const [[, me], userData] = await Promise.all([
      api.getMe(),
      getUser({username: cleanUsername}),
    ]);
    const currentUser = me?.user ?? null;
    const user = userData!.user;

    return (
      <DefaultLayout>
        <ProfileContainer>
          <ProfileHeader
            username={user.username}
            role={user.role}
            createdAt={user.createdAt}
            isOwnProfile={currentUser?.id === user.id}
          />
          <Suspense fallback={<UserContestsSkeleton />}>
            <UserContestsSection username={cleanUsername} page={userPage} />
          </Suspense>
        </ProfileContainer>
      </DefaultLayout>
    );
  }

  // Handle Organization profile (/{login})
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/${decoded}`);
  }

  const orgData = await getOrganization({login: decoded});
  const org = orgData.organization;

  const [
    [contestsError, contestsData],
    [, me],
  ] = await Promise.all([
    api.listWorkshopContests({page: currentPage, pageSize: 10, search, organizationId: org.id}),
    api.getMe(),
  ]);

  const currentUser = me?.user ?? null;
  const contests = contestsData?.contests ?? [];
  const contestsPagination = contestsData?.pagination ?? {page: 1, total: 1};
  const isAuthenticated = currentUser !== null;

  return (
    <Container size="lg" py="lg">
      {contestsError ? (
        <ErrorDisplay error={contestsError} />
      ) : (
        <OrgContestsTab
          contests={contests}
          pagination={contestsPagination}
          org={org}
          isAuthenticated={isAuthenticated}
          search={search}
        />
      )}
    </Container>
  );
};

export default Page;
