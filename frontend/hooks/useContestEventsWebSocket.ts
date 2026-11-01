"use client";

import {notifications} from "@mantine/notifications";
import {useEffect, useRef} from "react";

import {env} from "@/lib/env";

import type ContestMessage from "@/contracts/observer/v1/ContestMessage";

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
  const reconnectAttemptsRef = useRef(0);
  const latest = useRef({onEvent, currentUserId, isModerator});

  useEffect(() => {
    latest.current = {onEvent, currentUserId, isModerator};
  }, [onEvent, currentUserId, isModerator]);

  useEffect(() => {
    if (!enabled || !contestId) {
      return;
    }

    let disposed = false;
    let socket: WebSocket | null = null;
    let reconnectTimeout: ReturnType<typeof setTimeout> | undefined;

    const connect = () => {
      if (disposed) {
        return;
      }

      const wsBase = env.getWebSocketUrl();
      const wsUrl = `${wsBase}/contests?contestId=${contestId}`;

      try {
        const ws = new WebSocket(wsUrl);
        socket = ws;

        ws.onopen = () => {
          if (socket !== ws || disposed) {
            return;
          }
          reconnectAttemptsRef.current = 0;
        };

        ws.onmessage = (event) => {
          if (socket !== ws || disposed) {
            return;
          }
          try {
            const data: ContestMessage = JSON.parse(event.data);
            latest.current.onEvent?.(data);

            switch (data.event_type) {
              case "contest.announcement_created": {
                const payload = data.payload;
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

              case "contest.clarification_created": {
                const payload = data.payload;
                if (latest.current.isModerator && payload.user_id !== latest.current.currentUserId) {
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

              case "contest.clarification_answered": {
                const payload = data.payload;
                if (payload.user_id === latest.current.currentUserId) {
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
          if (disposed || socket !== ws) {
            return;
          }

          const attempts = reconnectAttemptsRef.current;
          const delay = Math.min(1000 * Math.pow(1.5, attempts), 10000);
          reconnectAttemptsRef.current += 1;

          reconnectTimeout = setTimeout(() => {
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
      disposed = true;
      clearTimeout(reconnectTimeout);
      if (socket) {
        socket.onopen = null;
        socket.onmessage = null;
        socket.onerror = null;
        socket.onclose = null;
        socket.close();
        socket = null;
      }
    };
  }, [contestId, enabled]);
};
