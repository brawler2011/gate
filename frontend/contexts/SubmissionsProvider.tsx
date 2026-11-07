import {useQueryClient, type Query} from '@tanstack/react-query';
import {createContext, useEffect, useState} from 'react';

import {useWebSocketNotifications} from '@/hooks/useWebSocketNotifications';
import {api} from '@/lib/api';
import {env} from '@/lib/env';
import {connectSubmissions, type SubmissionsConnectionStatus} from '@/lib/submissionsConnection';
import {applySubmissionEvent, applyScoreboardEvent} from '@/lib/submissionsUpdates';

import type SubmissionsMessage from '@/contracts/observer/v1/SubmissionsMessage';
import type {SubmissionsSelection} from '@/lib/submissions';
import type {LiveSubmissions, LiveScoreboard, SubmissionEvent} from '@/lib/submissionsUpdates';
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
    const revisions = new Map<string, number>();
    const progress = new Map<string, SubmissionEvent>();
    const highlightTimers = new Map<string, ReturnType<typeof setTimeout>>();
    let refreshTimer: ReturnType<typeof setTimeout> | undefined;
    let refreshing = false;
    let dirty = false;
    let reconnect = false;
    const applyProgress = (query: Query, replay = false) => {
      if (disposed) {
        return;
      }
      const selection = query.meta?.selection as SubmissionsSelection | undefined;
      if (!selection) {
        return;
      }
      client.setQueryData<LiveSubmissions>(query.queryKey, data => {
        for (const message of progress.values()) {
          data = applySubmissionEvent(data, message, selection, revisions, replay);
        }
        return data;
      });
    };
    // A completed HTTP snapshot can overwrite progress received before or during
    // the fetch. Reapply it only when its submission revision is still current.
    const unsubscribe = client.getQueryCache().subscribe(event => {
      if (event.type === 'updated' && event.action.type === 'success' && !event.action.manual
        && client.getQueryCache().findAll(submissions).includes(event.query)) {
        applyProgress(event.query, true);
      }
    });
    const refresh = async () => {
      refreshTimer = undefined;
      if (disposed || refreshing || !dirty) {
        return;
      }
      refreshing = true;
      dirty = false;
      const refreshScoreboard = reconnect;
      reconnect = false;
      try {
        if (refreshScoreboard) {
          await Promise.all([client.cancelQueries(submissions), client.cancelQueries(scoreboard)]);
        } else {
          // A fetch already in flight may predate the triggering event. Wait
          // for it, then request a fresh snapshot rather than reusing its result.
          await Promise.allSettled(client.getQueryCache().findAll(submissions).map(query => query.promise));
        }
        if (disposed) {
          return;
        }
        await Promise.all([
          client.invalidateQueries(submissions, {cancelRefetch: false}),
          ...(refreshScoreboard ? [client.invalidateQueries(scoreboard, {cancelRefetch: false})] : []),
        ]);
      } finally {
        refreshing = false;
        if (!disposed && dirty) {
          refreshTimer = setTimeout(() => {
            void refresh(); 
          }, 100);
        }
      }
    };
    const requestRefresh = (immediate = false) => {
      dirty = true;
      if (refreshing || refreshTimer !== undefined) {
        return;
      }
      if (immediate) {
        void refresh();
      } else {
        // Fixed windows keep continuous traffic from postponing refreshes.
        refreshTimer = setTimeout(() => {
          void refresh(); 
        }, 100);
      }
    };
    const disconnect = connectSubmissions({
      url: () => `${env.getWebSocketUrl()}/submissions?contestId=${encodeURIComponent(contestId)}`,
      onStatusChange: next => {
        setStatus(next);
        if (next.phase === 'connected') {
          reconnect = true;
          requestRefresh(true);
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
        if (message.event_type === 'scoreboard.updated') {
          if (message.payload.scoreboard?.contest_id !== contestId) {
            return;
          }
        } else if (!['submissions.created', 'submissions.completed', 'submissions.queued',
          'submissions.compiling_started', 'submissions.testing_started', 'submissions.test_started'].includes(message.event_type)
          || message.payload.contest_id !== contestId || typeof message.payload.id !== 'string') {
          return;
        }
        if (message.event_type === 'scoreboard.updated') {
          for (const query of client.getQueryCache().findAll(scoreboard)) {
            const apply = () => {
              if (disposed || Boolean(query.meta?.unfrozen) !== message.payload.unfrozen) {
                return;
              }
              client.setQueryData<LiveScoreboard>(query.queryKey,
                data => applyScoreboardEvent(data, message.payload, message.sequence));
            };
            apply();
            if (query.promise) {
              void query.promise.then(apply, () => {});
            }
          }
        } else if (typeof message.payload.revision === 'number') {
          const {id, revision} = message.payload;
          const latest = revisions.get(id) ?? 0;
          if (revision >= latest) {
            revisions.set(id, revision);
            if (message.event_type === 'submissions.created' || message.event_type === 'submissions.completed') {
              const buffered = progress.get(id);
              if (buffered && (buffered.payload.revision < revision || message.event_type === 'submissions.completed')) {
                progress.delete(id);
              }
            } else if (message.sequence > (progress.get(id)?.sequence ?? 0)) {
              progress.set(id, message);
              for (const query of client.getQueryCache().findAll(submissions)) {
                applyProgress(query);
              }
            }
          }
          if (message.event_type === 'submissions.created' || message.event_type === 'submissions.completed') {
            requestRefresh();
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
      unsubscribe();
      clearTimeout(refreshTimer);
      progress.clear();
      highlightTimers.forEach(clearTimeout);
    };
  }, [client, contestId, orgLogin, contestLogin, userId]);
  return <SubmissionsHighlights.Provider value={highlights}>{children}</SubmissionsHighlights.Provider>;
};
