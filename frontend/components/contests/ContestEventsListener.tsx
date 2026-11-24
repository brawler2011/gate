"use client";

import {useQueryClient} from "@tanstack/react-query";
import {type ReactNode} from "react";

import {useContestEventsWebSocket} from "@/hooks/useContestEventsWebSocket";
import {api} from "@/lib/api";

interface ContestEventsListenerProps {
  contestId: string;
  currentUserId?: string;
  isModerator?: boolean;
}

export const ContestEventsListener = ({
  contestId,
  currentUserId,
  isModerator = false,
}: ContestEventsListenerProps): ReactNode => {
  const queryClient = useQueryClient();
  useContestEventsWebSocket({
    contestId,
    enabled: !!currentUserId,
    currentUserId,
    isModerator,
    onConnected: () => {
      for (const resource of ['announcements', 'clarifications'] as const) {
        void queryClient.invalidateQueries({queryKey: api.queryOptions('get',
          `/contests/{contest_id}/${resource}`, {
            params: {path: {contest_id: contestId}},
          }).queryKey});
      }
    },
    onEvent: event => {
      const path = event.event_type === 'contest.announcement_created' || event.event_type === 'contest.announcement_deleted'
        ? '/contests/{contest_id}/announcements'
        : '/contests/{contest_id}/clarifications';
      void queryClient.invalidateQueries({queryKey: api.queryOptions('get', path, {
        params: {path: {contest_id: contestId}},
      }).queryKey});
    },
  });

  return null;
};
