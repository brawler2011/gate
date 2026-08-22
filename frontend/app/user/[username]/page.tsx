import {notFound, redirect} from "next/navigation";
import {Suspense} from "react";

import {DefaultLayout} from "@/components/shared";
import {
  ClaimTemporaryAccountSection,
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
  params: Promise<{ username: string }>;
  searchParams: Promise<{ contestsPage?: string }>;
};

const getUser = unwrapAndCache(api.getUser);

export const generateMetadata = async ({params}: Props): Promise<Metadata> => {
  const {username} = await params;
  let cleanUsername = "";
  try {
    cleanUsername = decodeURIComponent(username);
  } catch {
    notFound();
  }

  if (!cleanUsername) {
    notFound();
  }

  const data = await getUser({username: cleanUsername});
  return {title: `${data.user.username}`};
};

const Page = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {username} = await params;
  const {contestsPage} = await searchParams;

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

  const [[, me], userData] = await Promise.all([
    api.getMe(),
    getUser({username: cleanUsername}),
  ]);
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
