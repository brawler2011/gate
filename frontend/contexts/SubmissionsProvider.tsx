import {useQueryClient} from '@tanstack/react-query';
import {createContext, useEffect, useState} from 'react';

import {useWebSocketNotifications} from '@/hooks/useWebSocketNotifications';
import {api} from '@/lib/api';
import {env} from '@/lib/env';
import {connectSubmissions, type SubmissionsConnectionStatus} from '@/lib/submissionsConnection';
import {applySubmissionEvent, applyScoreboardEvent} from '@/lib/submissionsUpdates';

import type SubmissionsMessage from '@/contracts/observer/v1/SubmissionsMessage';
import type {SubmissionsSelection} from '@/lib/submissions';
import type {LiveSubmissions, LiveScoreboard} from '@/lib/submissionsUpdates';
import type {ReactNode} from 'react';

export const SubmissionsHighlights = createContext<ReadonlyMap<string, boolean>>(new Map());

export const SubmissionsProvider = ({contestId, orgLogin, contestLogin, userId, children}: {
  contestId: string; orgLogin: string; contestLogin: string; userId?: string; children: ReactNode;
}): ReactNode => {
  const client = useQueryClient();
  const [status, setStatus] = useState<SubmissionsConnectionStatus>({phase: 'idle'});
  const [highlights, setHighlights] = useState(new Map<string, boolean>());
  useWebSocketNotifications(status);

  useEffect(() => {
    setHighlights(new Map());
    if (!userId) {
      setStatus({phase: 'idle'});
      return;
    }
    const path = {org_login: orgLogin, contest_login: contestLogin};
    const submissions = {queryKey: api.queryOptions('get',
      '/organizations/{org_login}/contests/{contest_login}/submissions', {params: {path}}).queryKey};
    const scoreboard = {queryKey: api.queryOptions('get',
      '/organizations/{org_login}/contests/{contest_login}/scoreboard', {params: {path}}).queryKey};
    let disposed = false;
    const highlightTimers = new Map<string, ReturnType<typeof setTimeout>>();
    const refresh = async () => {
      await Promise.all([client.cancelQueries(submissions), client.cancelQueries(scoreboard)]);
      await Promise.all([client.invalidateQueries(submissions), client.invalidateQueries(scoreboard)]);
    };
    const disconnect = connectSubmissions({
      url: () => `${env.getWebSocketUrl()}/submissions?contestId=${encodeURIComponent(contestId)}`,
      onStatusChange: next => {
        setStatus(next);
        if (next.phase === 'connected') {
          void refresh();
        }
      },
      onMessage: event => {
        let message: SubmissionsMessage;
        try {
          message = JSON.parse(event.data);
        } catch {
          return;
        }
        if (!message?.payload || typeof message.sequence !== 'number') {
          return;
        }
        const isScoreboard = message.event_type === 'scoreboard.updated';
        if (message.event_type === 'scoreboard.updated') {
          if (message.payload.scoreboard?.contest_id !== contestId) {
            return;
          }
        } else if (!['submissions.created', 'submissions.completed', 'submissions.queued',
          'submissions.compiling_started', 'submissions.testing_started', 'submissions.test_started'].includes(message.event_type)
          || message.payload.contest_id !== contestId || typeof message.payload.id !== 'string') {
          return;
        }
        const queries = client.getQueryCache().findAll(isScoreboard ? scoreboard : submissions);
        for (const query of queries) {
          const apply = () => {
            if (disposed) {
              return;
            }
            if (message.event_type === 'scoreboard.updated') {
              if (Boolean(query.meta?.unfrozen) !== message.payload.unfrozen) {
                return;
              }
              client.setQueryData<LiveScoreboard>(query.queryKey,
                data => applyScoreboardEvent(data, message.payload, message.sequence));
            } else {
              const selection = query.meta?.selection as SubmissionsSelection | undefined;
              if (!selection) {
                return;
              }
              client.setQueryData<LiveSubmissions>(query.queryKey,
                data => applySubmissionEvent(data, message, selection));
            }
          };
          apply();
          // Reapply after an in-flight snapshot; its cursor excludes events it already contains.
          if (query.promise) {
            void query.promise.then(apply, () => {});
          }
        }
        if (message.event_type === 'scoreboard.updated') {
          return;
        }
        const {id} = message.payload;
        clearTimeout(highlightTimers.get(id));
        setHighlights(previous => new Map(previous).set(id, message.event_type === 'submissions.created' || previous.get(id) === true));
        highlightTimers.set(id, setTimeout(() => {
          highlightTimers.delete(id);
          setHighlights(previous => {
            const next = new Map(previous); next.delete(id); return next;
          });
        }, 2000));
      },
    });
    return () => {
      disposed = true;
      disconnect();
      highlightTimers.forEach(clearTimeout);
    };
  }, [client, contestId, orgLogin, contestLogin, userId]);
  return <SubmissionsHighlights.Provider value={highlights}>{children}</SubmissionsHighlights.Provider>;
};
