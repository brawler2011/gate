import {Badge, Box, Group, Switch, Table, Text, TextInput, Title, Tooltip} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconSearch} from "@tabler/icons-react";
import React, {useEffect, useRef, useState, useMemo} from "react";
import {Link} from "react-router-dom";

import {api} from "@/lib/api";
import {env} from "@/lib/env";
import {numberToLetters} from "@/lib/lib";
import {connectSubmissions} from "@/lib/submissionsConnection";

import classes from "./ContestMonitorTable.module.css";

import type {ScoreboardResponseModel, ScoreboardProblemResultModel} from "@/contracts/core/v1/schema";
import type SubmissionsMessage from "@/contracts/observer/v1/SubmissionsMessage";

interface ContestMonitorTableProps {
  contestId: string;
  orgLogin: string;
  contestLogin: string;
  initialScoreboard: ScoreboardResponseModel;
  startTime?: string | null;
  endTime?: string | null;
  isManager?: boolean;
}

const formatTimeMinutes = (timeMinutes?: number | null): string => {
  if (timeMinutes === undefined || timeMinutes === null) {
    return "";
  }
  const h = Math.floor(timeMinutes / 60);
  const m = timeMinutes % 60;
  return `${h.toString().padStart(2, "0")}:${m.toString().padStart(2, "0")}`;
};

export const ContestMonitorTable = ({
  contestId,
  orgLogin,
  contestLogin,
  initialScoreboard,
  isManager = false,
}: ContestMonitorTableProps): React.ReactNode => {
  const [searchQuery, setSearchQuery] = useState("");
  const [showRealMonitor, setShowRealMonitor] = useState(false);
  const scoreboard = api.useQuery('get', '/organizations/{org_login}/contests/{contest_login}/scoreboard', {
    params: {
      path: {org_login: orgLogin, contest_login: contestLogin},
      query: {unfrozen: showRealMonitor},
    },
  }, {initialData: showRealMonitor ? undefined : initialScoreboard, refetchOnMount: false});
  const {items, problems, is_frozen: isFrozen} = scoreboard.data ?? initialScoreboard;
  const refresh = useRef(scoreboard.refetch);
  useEffect(() => {
    refresh.current = scoreboard.refetch;
  }, [scoreboard.refetch]);

  useEffect(() => {
    if (scoreboard.error) {
      notifications.show({title: "Ошибка", message: "Не удалось загрузить таблицу", color: "red"});
    }
  }, [scoreboard.error]);

  useEffect(() => connectSubmissions({
    url: () => `${env.getWebSocketUrl()}/submissions?contestId=${contestId}`,
    onMessage: (event: MessageEvent) => {
      try {
        const msg: SubmissionsMessage = JSON.parse(event.data);
        if (msg.event_type === "submissions.completed" && msg.payload.contest_id === contestId) {
          void refresh.current();
        }
      } catch {
        // ignore parse error
      }
    },
    onStatusChange: status => {
      if (status.phase === 'connected') {
        void refresh.current();
      }
    },
  }), [contestId, orgLogin, contestLogin]);

  const filteredItems = useMemo(() => {
    if (!searchQuery.trim()) {
      return items;
    }
    const q = searchQuery.toLowerCase().trim();
    return items.filter((item) => item.username.toLowerCase().includes(q));
  }, [items, searchQuery]);

  const problemSummary = useMemo(() => {
    const solvedCounts: Record<string, number> = {};
    const attemptedCounts: Record<string, number> = {};

    for (const prob of problems) {
      solvedCounts[prob.problem_id] = 0;
      attemptedCounts[prob.problem_id] = 0;
    }

    for (const item of items) {
      for (const res of item.problem_results) {
        if (res.solved) {
          solvedCounts[res.problem_id] = (solvedCounts[res.problem_id] || 0) + 1;
          attemptedCounts[res.problem_id] = (attemptedCounts[res.problem_id] || 0) + 1;
        } else if ((res.failed_attempts || 0) > 0 || (res.pending_attempts || 0) > 0) {
          attemptedCounts[res.problem_id] = (attemptedCounts[res.problem_id] || 0) + 1;
        }
      }
    }

    return {solvedCounts, attemptedCounts};
  }, [items, problems]);

  return (
    <Box className={classes.container}>
      <Group justify="space-between" align="center" className={classes.topBar}>
        <Group align="center" gap="sm">
          <Title order={2} className={classes.title}>
            Положение
          </Title>
          {(isFrozen || initialScoreboard.is_frozen) && (
            <Badge color="orange" variant="light" size="lg">
              Монитор заморожен
            </Badge>
          )}
        </Group>
        <Group align="center" gap="md">
          {isManager && (initialScoreboard.is_frozen || isFrozen) && (
            <Switch
              size="sm"
              label={showRealMonitor ? "Реальный монитор" : "Замороженный вид"}
              checked={showRealMonitor}
              onChange={event => setShowRealMonitor(event.currentTarget.checked)}
              disabled={scoreboard.isFetching}
            />
          )}
          <TextInput
            placeholder="Найти участника"
            rightSection={<IconSearch size={16} />}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.currentTarget.value)}
            className={classes.searchInput}
          />
        </Group>
      </Group>

      <Box className={classes.tableWrapper}>
        <Table verticalSpacing="sm" className={classes.table}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th style={{width: "50px"}}>№</Table.Th>
              <Table.Th>Название</Table.Th>
              <Table.Th className={classes.thCenter} style={{width: "50px"}}>
                =
              </Table.Th>
              <Table.Th className={classes.thCenter} style={{width: "80px"}}>
                Штраф
              </Table.Th>
              {problems.map((p) => {
                const letter = numberToLetters(p.ordinal);
                return (
                  <Table.Th key={p.problem_id} className={classes.thCenter} style={{width: "70px"}}>
                    <Tooltip label={p.title} withArrow>
                      <Link
                        to={`/${orgLogin}/${contestLogin}/${letter}`}
                        style={{color: "inherit", textDecoration: "none"}}
                      >
                        <span>{letter}</span>
                      </Link>
                    </Tooltip>
                  </Table.Th>
                );
              })}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {filteredItems.map((item, index) => {
              const resultMap = new Map<string, ScoreboardProblemResultModel>();
              for (const r of item.problem_results) {
                resultMap.set(r.problem_id, r);
              }

              return (
                <Table.Tr key={item.user_id}>
                  <Table.Td>{index + 1}</Table.Td>
                  <Table.Td>
                    <Text fw={500} size="sm">
                      {item.username}
                    </Text>
                  </Table.Td>
                  <Table.Td className={classes.tdCenter}>
                    <Text fw={700} size="sm">
                      {item.problems_solved}
                    </Text>
                  </Table.Td>
                  <Table.Td className={classes.tdCenter}>
                    <Text size="sm">{item.total_penalty}</Text>
                  </Table.Td>
                  {problems.map((prob) => {
                    const res = resultMap.get(prob.problem_id);
                    if (!res) {
                      return <Table.Td key={prob.problem_id} className={classes.tdCenter} />;
                    }

                    const hasPending = (res.pending_attempts || 0) > 0;
                    const failed = res.failed_attempts || 0;
                    const pending = res.pending_attempts || 0;
                    const timeStr = formatTimeMinutes(res.time_minutes);

                    if (res.solved) {
                      let label = failed > 0 ? `+${failed}` : "+";
                      if (hasPending) {
                        label = `${label} ${pending}?`;
                      }
                      const cellClass = hasPending ? classes.cellFrozenPending : classes.cellSolved;
                      return (
                        <Table.Td key={prob.problem_id} className={classes.tdCenter}>
                          <div className={cellClass}>
                            <span>{label}</span>
                            {timeStr && <span className={classes.cellTime}>{timeStr}</span>}
                          </div>
                        </Table.Td>
                      );
                    }

                    if (hasPending) {
                      let label = `?${pending}`;
                      if (failed > 0) {
                        label = `-${failed} ${pending}?`;
                      }
                      return (
                        <Table.Td key={prob.problem_id} className={classes.tdCenter}>
                          <div className={classes.cellFrozenPending}>
                            <span>{label}</span>
                          </div>
                        </Table.Td>
                      );
                    }

                    if (failed > 0) {
                      return (
                        <Table.Td key={prob.problem_id} className={classes.tdCenter}>
                          <div className={classes.cellFailed}>
                            <span>-{failed}</span>
                          </div>
                        </Table.Td>
                      );
                    }

                    return <Table.Td key={prob.problem_id} className={classes.tdCenter} />;
                  })}
                </Table.Tr>
              );
            })}

            {/* Bottom summary rows */}
            <Table.Tr>
              <Table.Td />
              <Table.Td>
                <Text className={classes.summaryLabelSolved}>Сдали</Text>
              </Table.Td>
              <Table.Td />
              <Table.Td />
              {problems.map((prob) => (
                <Table.Td key={prob.problem_id} className={classes.tdCenter}>
                  <Text className={classes.summaryLabelSolved}>
                    {problemSummary.solvedCounts[prob.problem_id] || 0}
                  </Text>
                </Table.Td>
              ))}
            </Table.Tr>
            <Table.Tr>
              <Table.Td />
              <Table.Td>
                <Text className={classes.summaryLabelAttempted}>Пытались</Text>
              </Table.Td>
              <Table.Td />
              <Table.Td />
              {problems.map((prob) => (
                <Table.Td key={prob.problem_id} className={classes.tdCenter}>
                  <Text className={classes.summaryLabelAttempted}>
                    {problemSummary.attemptedCounts[prob.problem_id] || 0}
                  </Text>
                </Table.Td>
              ))}
            </Table.Tr>
          </Table.Tbody>
        </Table>
      </Box>
    </Box>
  );
};
