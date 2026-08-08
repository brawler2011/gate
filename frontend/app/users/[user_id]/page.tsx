import { notFound } from "next/navigation";

import { DefaultLayout } from '@/components/shared';
import { Profile } from '@/components/users/Profile';
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
  const {user_id} = await params;

  const userId = parseId(user_id);
  if (!userId) {
    notFound();
  }

  const data = await getUser({id: userId});

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

  const [[, me], userData, [, contestsData]] = await Promise.all([
    api.getMe(),
    getUser({id: userId}),
    api.listUserContests({ id: userId, page: page, pageSize: 10 }),
  ]);
  const currentUser = me?.user ?? null;

  const user = userData!.user;

  return (
    <DefaultLayout>
      <Profile
        username={user.username}
        role={user.role}
        createdAt={user.createdAt}
        userId={user_id}
        contests={contestsData?.contests ?? []}
        contestsPagination={contestsData?.pagination}
        contestsPage={page}
        isOwnProfile={currentUser?.id === user_id}
      />
    </DefaultLayout>
  );
};

export default Page;
