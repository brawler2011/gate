import {api} from "@/lib/api";

import {ProfileContests} from "./ProfileContests";

type UserContestsSectionProps = {
  userId: string;
  page: number;
};

export const UserContestsSection = async ({userId, page}: UserContestsSectionProps) => {
  const [, contestsData] = await api.listUserContests({id: userId, page, pageSize: 10});

  return (
    <ProfileContests
      userId={userId}
      contests={contestsData?.contests ?? []}
      contestsPagination={contestsData?.pagination}
      contestsPage={page}
    />
  );
};
