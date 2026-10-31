import {Suspense} from "react";
import {useLoaderData, redirect} from "react-router-dom";

import {DefaultLayout} from "@/components/shared";
import {ClaimTemporaryAccountSection, ProfileContainer, ProfileHeader, UserContestsSection, UserContestsSkeleton} from "@/components/users";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams, request}: LoaderFunctionArgs) => {
  const searchParams = Object.fromEntries(new URL(request.url).searchParams);
  const {username} = routeParams as {username: string};
  const {contestsPage} = searchParams;

  const cleanUsername = username;

  if (!cleanUsername) {
    throw new Response("Страница не найдена", {status: 404});
  }

  const userPage = parsePage(contestsPage);
  if (!userPage) {
    throw redirect(`/@${cleanUsername}`);
  }

  const [[, me], userData] = await Promise.all([api.getMe(), getUser(cleanUsername)]);
  return {cleanUsername, userPage, me, userData};
}) satisfies LoaderFunction;

const getUser = unwrap(api.getUser);

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {cleanUsername, userPage, me, userData} = data;
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
