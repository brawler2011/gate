import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {useState} from 'react';

import {ContestMonitorTable} from '@/components/contests/ContestMonitorTable';
import {SubmissionsProvider} from '@/contexts/SubmissionsProvider';
import {api} from '@/lib/api';

import type {ScoreboardResponseModel} from '@/contracts/core/v1/schema';
import type {ComponentProps, ReactNode} from 'react';

export const Monitor = ({initialScoreboard, ...props}: ComponentProps<typeof ContestMonitorTable> & {
  initialScoreboard: ScoreboardResponseModel;
}): ReactNode => {
  const [client] = useState(() => {
    const client = new QueryClient({defaultOptions: {queries: {retry: false, staleTime: 10000, gcTime: 0}}});
    client.setQueryData(api.queryOptions('get', '/organizations/{org_login}/contests/{contest_login}/scoreboard', {
      params: {path: {org_login: props.orgLogin, contest_login: props.contestLogin}, query: {unfrozen: false}},
    }).queryKey, initialScoreboard);
    return client;
  });
  return <QueryClientProvider client={client}>
    <SubmissionsProvider contestId="contest" orgLogin={props.orgLogin} contestLogin={props.contestLogin} userId="user">
      <ContestMonitorTable {...props} />
    </SubmissionsProvider>
  </QueryClientProvider>;
};
