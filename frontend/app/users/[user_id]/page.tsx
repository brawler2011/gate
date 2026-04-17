import { DefaultLayout } from '@/components/shared';
import { Profile } from '@/components/users/Profile';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { getUser, getUserContests } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { isValidUUIDV4 } from "@/lib/lib";
import { Metadata } from "next";
import { notFound } from "next/navigation";

type Props = {
  params: Promise<{ user_id: string }>;
  searchParams: Promise<{ contestsPage?: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const resolvedParams = await params;
  const user_id = resolvedParams.user_id;

  if (!user_id || !isValidUUIDV4(user_id)) {
    return { title: "Профиль пользователя - Gate149" };
  }

  const [error, userData] = await getUser(user_id);
  if (error || !userData) {
    return { title: "Ошибка загрузки профиля - Gate149" };
  }

  return { title: `${userData.user.username} - Gate149` };
}

const Page = async ({ params, searchParams }: Props) => {
  const { user_id } = await params;
  const { contestsPage } = await searchParams;
  const contestsPageNum = Number(contestsPage) || 1;

  if (!user_id || !isValidUUIDV4(user_id)) {
    notFound();
  }

  const [currentUser, [userError, userData], [, contestsData]] = await Promise.all([
    getCurrentUser(),
    getUser(user_id),
    getUserContests(user_id, contestsPageNum, 10),
  ]);

  if (userError) return <ErrorDisplay error={userError} />;

  const user = userData!.user;

  return (
    <DefaultLayout>
      <Profile
        username={user.username}
        name={user.name}
        surname={user.surname}
        role={user.role}
        bio={user.bio}
        createdAt={user.createdAt}
        userId={user_id}
        contests={contestsData?.contests ?? []}
        contestsPagination={contestsData?.pagination}
        contestsPage={contestsPageNum}
        isOwnProfile={currentUser?.id === user_id}
      />
    </DefaultLayout>
  );
};

export default Page;
