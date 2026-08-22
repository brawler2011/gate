"use client";

import {type ReactNode} from "react";

import {useContestEventsWebSocket} from "@/hooks/useContestEventsWebSocket";

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
  useContestEventsWebSocket({
    contestId,
    enabled: !!currentUserId,
    currentUserId,
    isModerator,
  });

  return null;
};
