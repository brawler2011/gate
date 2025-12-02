import { DefaultLayout } from "@/components/Layout";
import { Profile } from "@/components/Profile";
import { getUser } from "@/lib/actions";
import { isValidUUIDV4 } from "@/lib/lib";
import { Metadata } from "next";
import { notFound } from "next/navigation";

type Props = {
  params: Promise<{ user_id: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const resolvedParams = await params;
  const user_id = resolvedParams.user_id;

  if (!user_id || !isValidUUIDV4(user_id)) {
    return {
      title: "Профиль пользователя - Gate149",
    };
  }

  try {
    const userData = await getUser(user_id);
    return {
      title: `${userData.user.username} - Gate149`,
    };
  } catch {
    return {
      title: "Ошибка загрузки профиля - Gate149",
    };
  }
}

const Page = async ({ params }: Props) => {
  const resolvedParams = await params;
  const user_id = resolvedParams.user_id;

  if (!user_id || !isValidUUIDV4(user_id)) {
    notFound();
  }

  const userData = await getUser(user_id);

  return (
    <DefaultLayout>
      <Profile 
        username={userData.user.username}
        email={"kotok.9647@gmail.com"}
        name={"Алексей"}
        surname={"Котоков"}
        role={userData.user.role}
        avatarlink={"/images/avatar.png"}
      />
    </DefaultLayout>
  );
};

export default Page;
