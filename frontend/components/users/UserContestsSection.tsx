import {useListUserContests} from '@/contracts/core/v1/api';

import {ProfileContests} from './ProfileContests';

import type {ReactNode} from 'react';

type UserContestsSectionProps = {username: string; page: number};
export const UserContestsSection = ({username, page}: UserContestsSectionProps): ReactNode => {
  const {data} = useListUserContests(username, {page, pageSize: 10});
  return <ProfileContests username={username} contests={data?.contests ?? []}
    contestsPagination={data?.pagination} contestsPage={page} />;
};
