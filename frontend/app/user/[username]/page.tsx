import {Suspense} from "react";

import {DefaultLayout} from "@/components/shared";
import {ClaimTemporaryAccountSection, ProfileContainer, ProfileHeader, UserContestsSection, UserContestsSkeleton} from "@/components/users";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {notFound, redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadPage = async ({params, searchParams}: Props) => {
  const {username} = params;
  const {contestsPage} = searchParams;

  let cleanUsername = "";
  try {
    cleanUsername = decodeURIComponent(username);
  } catch {
    notFound();
  }

  if (!cleanUsername) {
    notFound();
  }

  const userPage = parsePage(contestsPage);
  if (!userPage) {
    redirect(`/@${cleanUsername}`);
  }

  const [[, me], userData] = await Promise.all([api.getMe(), getUser(cleanUsername)]);
  return {username, contestsPage, cleanUsername, userPage, me, userData};
};

type Props = {
  params: { username: string };
  searchParams: { contestsPage?: string };
};

const getUser = unwrap(api.getUser);

const Page = ({params, searchParams}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/user/[username]/page.tsx", params, searchParams], () =>
    loadPage({params, searchParams}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {cleanUsername, userPage, me, userData} = routeQuery.data;
  const currentUser = me?.user ?? null;
  const user = userData.user;
  return (
    <DefaultLayout headerUser={currentUser}>
      <ProfileContainer>
        <ProfileHeader
          username={user.username}
          role={user.role}
          createdAt={user.createdAt}
          expiresAt={user.expires_at}
          claimedAt={user.claimed_at}
          isOwnProfile={currentUser?.id === user.id}
        />
        {currentUser?.id === user.id && <ClaimTemporaryAccountSection />}
        <Suspense fallback={<UserContestsSkeleton />}>
          <UserContestsSection username={cleanUsername} page={userPage} />
        </Suspense>
      </ProfileContainer>
    </DefaultLayout>
  );
};

export default Page;
