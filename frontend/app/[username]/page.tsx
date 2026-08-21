import {notFound, redirect} from "next/navigation";
import {Suspense} from "react";

import {DefaultLayout} from "@/components/shared";
import {
  ProfileContainer,
  ProfileHeader,
  UserContestsSection,
  UserContestsSkeleton,
} from "@/components/users";
import {api, unwrapAndCache} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {Metadata} from "next";

type Props = {
  params: Promise<{ username: string }>;
  searchParams: Promise<{ contestsPage?: string }>;
};

const getUser = unwrapAndCache(api.getUser);

const parseUsername = (raw: string): string | null => {
  try {
    const decoded = decodeURIComponent(raw);
    if (!decoded.startsWith("@")) {
      return null;
    }
    const clean = decoded.slice(1);
    return clean.length > 0 ? clean : null;
  } catch {
    return null;
  }
};

export const generateMetadata = async ({params}: Props): Promise<Metadata> => {
  const {username} = await params;
  const cleanUsername = parseUsername(username);

  if (!cleanUsername) {
    notFound();
  }

  const data = await getUser({username: cleanUsername});

  return {title: `${data.user.username}`};
};

const Page = async ({params, searchParams}: Props): Promise<JSX.Element> => {
  const {username} = await params;
  const {contestsPage} = await searchParams;

  const cleanUsername = parseUsername(username);
  if (!cleanUsername) {
    notFound();
  }

  const page = parsePage(contestsPage);
  if (!page) {
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
          <UserContestsSection username={cleanUsername} page={page} />
        </Suspense>
      </ProfileContainer>
    </DefaultLayout>
  );
};

export default Page;
