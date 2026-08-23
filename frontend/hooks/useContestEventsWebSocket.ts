"use client";

import {notifications} from "@mantine/notifications";
import {useEffect, useRef} from "react";

import {
  ContestEventType,
  type ContestMessage,
  type MessageContestAnnouncementCreated,
  type MessageContestClarificationCreated,
  type MessageContestClarificationAnswered,
} from "@/contracts/observer/v1";
import {env} from "@/lib/env";

export interface UseContestEventsWebSocketProps {
  contestId?: string;
  enabled?: boolean;
  currentUserId?: string;
  isModerator?: boolean;
  onEvent?: (event: ContestMessage) => void;
}

export const useContestEventsWebSocket = ({
  contestId,
  enabled = true,
  currentUserId,
  isModerator,
  onEvent,
}: UseContestEventsWebSocketProps): void => {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const isUnmountedRef = useRef(false);

  const onEventRef = useRef(onEvent);
  const currentUserIdRef = useRef(currentUserId);
  const isModeratorRef = useRef(isModerator);

  useEffect(() => {
    onEventRef.current = onEvent;
    currentUserIdRef.current = currentUserId;
    isModeratorRef.current = isModerator;
  }, [onEvent, currentUserId, isModerator]);

  useEffect(() => {
    isUnmountedRef.current = false;

    if (!enabled || !contestId) {
      return;
    }

    const connect = () => {
      if (isUnmountedRef.current) {
        return;
      }

      const wsBase = env.getWebSocketUrl();
      const wsUrl = `${wsBase}/contests?contestId=${contestId}`;

      try {
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
          if (wsRef.current !== ws || isUnmountedRef.current) {
            return;
          }
          reconnectAttemptsRef.current = 0;
        };

        ws.onmessage = (event) => {
          if (wsRef.current !== ws || isUnmountedRef.current) {
            return;
          }
          try {
            const data: ContestMessage = JSON.parse(event.data);
            onEventRef.current?.(data);

            switch (data.event_type) {
              case ContestEventType.CONTEST_ANNOUNCEMENT_CREATED: {
                const payload = data.payload as MessageContestAnnouncementCreated;
                const problemInfo = payload.problem_letter
                  ? ` [Задача ${payload.problem_letter}]`
                  : "";

                notifications.show({
                  title: `Объявление жюри${problemInfo}`,
                  message: payload.title || "Новое сообщение от жюри",
                  color: "blue",
                  autoClose: 10000,
                  withCloseButton: true,
                });
                break;
              }

              case ContestEventType.CONTEST_CLARIFICATION_CREATED: {
                const payload = data.payload as MessageContestClarificationCreated;
                if (isModeratorRef.current && payload.user_id !== currentUserIdRef.current) {
                  const author = payload.username ? ` от ${payload.username}` : "";
                  const prob = payload.problem_letter
                    ? ` (Задача ${payload.problem_letter})`
                    : "";

                  notifications.show({
                    title: `Новый вопрос${author}${prob}`,
                    message: payload.question,
                    color: "yellow",
                    autoClose: 8000,
                    withCloseButton: true,
                  });
                }
                break;
              }

              case ContestEventType.CONTEST_CLARIFICATION_ANSWERED: {
                const payload = data.payload as MessageContestClarificationAnswered;
                if (payload.user_id === currentUserIdRef.current) {
                  const prob = payload.problem_letter
                    ? ` (Задача ${payload.problem_letter})`
                    : "";

                  notifications.show({
                    title: `Ответ на ваш вопрос${prob}`,
                    message: payload.answer || "Жюри ответило на ваш вопрос",
                    color: "teal",
                    autoClose: 12000,
                    withCloseButton: true,
                  });
                }
                break;
              }

              default:
                break;
            }
          } catch (err) {
            if (env.isDevelopment()) {
              console.error("Failed to parse contest WS event", err);
            }
          }
        };

        ws.onclose = () => {
          if (isUnmountedRef.current || wsRef.current !== ws) {
            return;
          }

          const attempts = reconnectAttemptsRef.current;
          const delay = Math.min(1000 * Math.pow(1.5, attempts), 10000);
          reconnectAttemptsRef.current += 1;

          reconnectTimeoutRef.current = setTimeout(() => {
            connect();
          }, delay);
        };

        ws.onerror = (err) => {
          if (env.isDevelopment()) {
            console.warn("Contest events WebSocket error", err);
          }
        };
      } catch (err) {
        if (env.isDevelopment()) {
          console.warn("Failed to establish contest WebSocket connection", err);
        }
      }
    };

    connect();

    return () => {
      isUnmountedRef.current = true;
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
      if (wsRef.current) {
        const ws = wsRef.current;
        wsRef.current = null;
        ws.onopen = null;
        ws.onmessage = null;
        ws.onerror = null;
        ws.onclose = null;
        ws.close();
      }
    };
  }, [contestId, enabled]);
};
