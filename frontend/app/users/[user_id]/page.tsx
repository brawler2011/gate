import { notFound } from "next/navigation";
import { Suspense } from "react";

import { DefaultLayout } from '@/components/shared';
import {
  ProfileContainer,
  ProfileHeader,
  UserContestsSection,
  UserContestsSkeleton,
} from '@/components/users';
import { api } from "@/lib/api";
import { unwrapAndCache } from "@/lib/api2";
import { parseId, parsePage } from "@/lib/lib2";

import type { Metadata } from "next";

type Props = {
  params: Promise<{ user_id: string }>;
  searchParams: Promise<{ contestsPage?: string }>;
};

const getUser = unwrapAndCache(api.getUser);

export const generateMetadata = async ({ params }: Props): Promise<Metadata> => {
  const { user_id } = await params;

  const userId = parseId(user_id);
  if (!userId) {
    notFound();
  }

  const data = await getUser({ id: userId });

  return { title: `${data.user.username}` };
};

const Page = async ({ params, searchParams }: Props) => {
  const { user_id } = await params;
  const { contestsPage } = await searchParams;
  const page = parsePage(contestsPage);

  const userId = parseId(user_id);
  if (!userId || !page) {
    notFound();
  }

  const [[, me], userData] = await Promise.all([
    api.getMe(),
    getUser({ id: userId }),
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
          isOwnProfile={currentUser?.id === user_id}
        />
        <Suspense fallback={<UserContestsSkeleton />}>
          <UserContestsSection userId={user_id} page={page} />
        </Suspense>
      </ProfileContainer>
    </DefaultLayout>
  );
};

export default Page;
