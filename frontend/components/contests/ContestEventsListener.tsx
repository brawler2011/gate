"use client";

import {useQueryClient} from "@tanstack/react-query";
import {type ReactNode} from "react";

import {useContestEventsWebSocket} from "@/hooks/useContestEventsWebSocket";
import {api} from "@/lib/api";

interface ContestEventsListenerProps {
  contestId: string;
  orgLogin: string;
  contestLogin: string;
  currentUserId?: string;
  isModerator?: boolean;
}

export const ContestEventsListener = ({
  contestId,
  orgLogin,
  contestLogin,
  currentUserId,
  isModerator = false,
}: ContestEventsListenerProps): ReactNode => {
  const queryClient = useQueryClient();
  useContestEventsWebSocket({
    contestId,
    enabled: !!currentUserId,
    currentUserId,
    isModerator,
    onEvent: event => {
      const path = event.event_type === 'contest.announcement_created' || event.event_type === 'contest.announcement_deleted'
        ? '/organizations/{org_login}/contests/{contest_login}/announcements'
        : '/organizations/{org_login}/contests/{contest_login}/clarifications';
      void queryClient.invalidateQueries({queryKey: api.queryOptions('get', path, {
        params: {path: {org_login: orgLogin, contest_login: contestLogin}},
      }).queryKey});
    },
  });

  return null;
};
