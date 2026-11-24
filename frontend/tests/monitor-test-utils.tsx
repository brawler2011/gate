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
    client.setQueryData(api.queryOptions('get', '/contests/{contest_id}/scoreboard', {
      params: {path: {contest_id: props.contestId}, query: {unfrozen: false}},
    }).queryKey, initialScoreboard);
    return client;
  });
  return <QueryClientProvider client={client}>
    <SubmissionsProvider contestId={props.contestId} orgLogin={props.orgLogin} contestLogin={props.contestLogin} userId="user">
      <ContestMonitorTable {...props} />
    </SubmissionsProvider>
  </QueryClientProvider>;
};
