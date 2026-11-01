import {useQueryClient} from '@tanstack/react-query';
import {createContext, useEffect, useState} from 'react';

import {useWebSocketNotifications} from '@/hooks/useWebSocketNotifications';
import {api} from '@/lib/api';
import {env} from '@/lib/env';
import {connectSubmissions, type SubmissionsConnectionStatus} from '@/lib/submissionsConnection';

import type {ListSubmissionsResponseModel} from '@/contracts/core/v1/schema';
import type SubmissionsMessage from '@/contracts/observer/v1/SubmissionsMessage';
import type {SubmissionWithProgress} from '@/lib/submissions';
import type {Query} from '@tanstack/react-query';
import type {ReactNode} from 'react';

type LiveSubmissions = Omit<ListSubmissionsResponseModel, 'submissions'> & {submissions: SubmissionWithProgress[]};
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
    let refreshAll = false;
    let refreshTimer: ReturnType<typeof setTimeout> | undefined;
    const highlightTimers = new Map<string, ReturnType<typeof setTimeout>>();
    const refresh = async (stateOnly = false) => {
      const filters = {...submissions, predicate: (query: Query) => !stateOnly || query.meta?.state !== undefined};
      await Promise.all([client.cancelQueries(filters), stateOnly ? undefined : client.cancelQueries(scoreboard)]);
      await Promise.all([
        client.invalidateQueries(filters), stateOnly ? undefined : client.invalidateQueries(scoreboard),
      ]);
    };
    const scheduleRefresh = (all: boolean) => {
      refreshAll ||= all;
      if (refreshTimer === undefined) {
        refreshTimer = setTimeout(() => {
          refreshTimer = undefined;
          const all = refreshAll;
          refreshAll = false;
          void refresh(!all);
        }, 100);
      }
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
        if (!message?.payload || message.payload.contest_id !== contestId || typeof message.payload.id !== 'string') {
          return;
        }
        const {id} = message.payload;
        let progress: SubmissionWithProgress['progress'];
        switch (message.event_type) {
          case 'submissions.created':
          case 'submissions.completed':
            scheduleRefresh(true);
            break;
          case 'submissions.queued': progress = {phase: 'queued'}; break;
          case 'submissions.compiling_started': progress = {phase: 'compiling'}; break;
          case 'submissions.testing_started': progress = {phase: 'testing'}; break;
          case 'submissions.test_started': progress = {phase: 'testing', testNumber: message.payload.number}; break;
          default: return;
        }
        if (progress) {
          scheduleRefresh(false);
        }
        client.setQueriesData<LiveSubmissions>(submissions, data => data && ({...data,
          submissions: data.submissions.map(s => s.id === id ? {...s, progress} : s),
        }));
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
      disconnect();
      clearTimeout(refreshTimer);
      highlightTimers.forEach(clearTimeout);
    };
  }, [client, contestId, orgLogin, contestLogin, userId]);
  return <SubmissionsHighlights.Provider value={highlights}>{children}</SubmissionsHighlights.Provider>;
};
