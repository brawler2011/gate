import {api} from "@/lib/api";

import {ProfileContests} from "./ProfileContests";

type UserContestsSectionProps = {
  username: string;
  page: number;
};

export const UserContestsSection = async ({username, page}: UserContestsSectionProps): Promise<JSX.Element> => {
  const [, contestsData] = await api.listUserContests({username, page, pageSize: 10});

  return (
    <ProfileContests
      username={username}
      contests={contestsData?.contests ?? []}
      contestsPagination={contestsData?.pagination}
      contestsPage={page}
    />
  );
};
