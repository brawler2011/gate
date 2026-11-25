import {Badge, Box, Group, Switch, Table, Text, TextInput, Title, Tooltip} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconSearch} from "@tabler/icons-react";
import {keepPreviousData} from "@tanstack/react-query";
import React, {useEffect, useState, useMemo} from "react";
import {Link} from "react-router-dom";

import {QueryState} from "@/components/shared/QueryState";
import {useVirtualRun} from "@/hooks/useVirtualRun";
import {api} from "@/lib/api";
import {numberToLetters} from "@/lib/lib";

import classes from "./ContestMonitorTable.module.css";

import type {ScoreboardProblemResultModel} from "@/contracts/core/v1/schema";

interface ContestMonitorTableProps {
  orgLogin: string;
  contestLogin: string;
  contestId: string;
  isManager?: boolean;
  allowVirtual?: boolean;
}

const formatTimeMinutes = (timeMinutes?: number | null): string => {
  if (timeMinutes === undefined || timeMinutes === null) {
    return "";
  }
  const h = Math.floor(timeMinutes / 60);
  const m = timeMinutes % 60;
  return `${h.toString().padStart(2, "0")}:${m.toString().padStart(2, "0")}`;
};

const formatScore = (score: number): string => Number.isInteger(score) ? String(score) : score.toFixed(2);

const PointsCell = ({result}: {result: ScoreboardProblemResultModel}): React.ReactNode => {
  const score = result.score ?? 0;
  const pending = result.pending_attempts || 0;
  if (score === 0 && !result.failed_attempts && !pending) {
    return null;
  }
  let cellClass = score > 0 ? classes.cellPartial : classes.cellFailed;
  if (pending > 0) {
    cellClass = classes.cellFrozenPending;
  } else if (result.solved) {
    cellClass = classes.cellSolved;
  }
  const timeStr = score > 0 ? formatTimeMinutes(result.time_minutes) : "";
  return (
    <div className={cellClass}>
      <span>{pending > 0 ? `${formatScore(score)} ${pending}?` : formatScore(score)}</span>
      {timeStr && <span className={classes.cellTime}>{timeStr}</span>}
    </div>
  );
};

export const ContestMonitorTable = ({
  orgLogin,
  contestLogin,
  contestId,
  isManager = false,
  allowVirtual = false,
}: ContestMonitorTableProps): React.ReactNode => {
  const [searchQuery, setSearchQuery] = useState("");
  const [showRealMonitor, setShowRealMonitor] = useState(false);
  const [showVirtual, setShowVirtual] = useState(false);
  const {replaying, pending, known} = useVirtualRun(orgLogin, contestLogin);
  const polled = !known || replaying || showVirtual;
  // Live updates carry official rows at the present moment, so replayed and virtual views poll instead.
  const scoreboard = api.useQuery('get', '/contests/{contest_id}/scoreboard', {
    params: {
      path: {contest_id: contestId},
      query: showVirtual ? {unfrozen: showRealMonitor, virtual: true} : {unfrozen: showRealMonitor},
    },
  }, {
    enabled: !pending,
    placeholderData: keepPreviousData,
    meta: {unfrozen: showRealMonitor, polled},
    refetchInterval: polled ? 30_000 : false,
  });
  useEffect(() => {
    if (scoreboard.error) {
      notifications.show({title: "Ошибка", message: "Не удалось загрузить таблицу", color: "red"});
    }
  }, [scoreboard.error]);

  const filteredItems = useMemo(() => {
    const items = scoreboard.data?.items ?? [];
    if (!searchQuery.trim()) {
      return items;
    }
    const q = searchQuery.toLowerCase().trim();
    return items.filter((item) => item.username.toLowerCase().includes(q));
  }, [scoreboard.data, searchQuery]);

  const problemSummary = useMemo(() => {
    const items = scoreboard.data?.items ?? [];
    const problems = scoreboard.data?.problems ?? [];
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
  }, [scoreboard.data]);

  if (!scoreboard.data) {
    return <QueryState error={scoreboard.error} />;
  }
  const {problems, is_frozen: isFrozen} = scoreboard.data;
  const byPoints = scoreboard.data.ranking === "points";
  return (
    <Box className={classes.container}>
      <Group justify="space-between" align="center" className={classes.topBar}>
        <Group align="center" gap="sm">
          <Title order={2} className={classes.title}>
            Положение
          </Title>
          {isFrozen && (
            <Badge color="orange" variant="light" size="lg">
              Монитор заморожен
            </Badge>
          )}
          {replaying && (
            <Badge color="violet" variant="light" size="lg">
              На тот же момент от начала
            </Badge>
          )}
        </Group>
        <Group align="center" gap="md">
          {isManager && (isFrozen || showRealMonitor || scoreboard.isPlaceholderData) && (
            <Switch
              size="sm"
              label={showRealMonitor ? "Реальный монитор" : "Замороженный вид"}
              checked={showRealMonitor}
              onChange={event => setShowRealMonitor(event.currentTarget.checked)}
              disabled={scoreboard.isFetching}
            />
          )}
          {allowVirtual && (
            <Switch
              size="sm"
              label="Виртуальные участники"
              checked={showVirtual}
              onChange={event => setShowVirtual(event.currentTarget.checked)}
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
              {byPoints ? (
                <Table.Th className={classes.thCenter} style={{width: "70px"}}>
                  Баллы
                </Table.Th>
              ) : (
                <>
                  <Table.Th className={classes.thCenter} style={{width: "50px"}}>
                    =
                  </Table.Th>
                  <Table.Th className={classes.thCenter} style={{width: "80px"}}>
                    Штраф
                  </Table.Th>
                </>
              )}
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
                        {byPoints && <span className={classes.cellTime}>{formatScore(p.max_score)}</span>}
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
                <Table.Tr key={`${item.user_id}:${item.virtual ? "virtual" : "official"}`}>
                  <Table.Td>{item.rank || index + 1}</Table.Td>
                  <Table.Td>
                    <Group gap={6} wrap="nowrap">
                      <Text fw={500} size="sm">
                        {item.username}
                      </Text>
                      {item.virtual && <Badge size="xs" variant="light" color="violet" tt="none">виртуально</Badge>}
                    </Group>
                  </Table.Td>
                  {byPoints ? (
                    <Table.Td className={classes.tdCenter}>
                      <Text fw={700} size="sm">
                        {formatScore(item.score ?? 0)}
                      </Text>
                    </Table.Td>
                  ) : (
                    <>
                      <Table.Td className={classes.tdCenter}>
                        <Text fw={700} size="sm">
                          {item.problems_solved}
                        </Text>
                      </Table.Td>
                      <Table.Td className={classes.tdCenter}>
                        <Text size="sm">{item.total_penalty}</Text>
                      </Table.Td>
                    </>
                  )}
                  {problems.map((prob) => {
                    const res = resultMap.get(prob.problem_id);
                    if (!res) {
                      return <Table.Td key={prob.problem_id} className={classes.tdCenter} />;
                    }
                    if (byPoints) {
                      return (
                        <Table.Td key={prob.problem_id} className={classes.tdCenter}>
                          <PointsCell result={res} />
                        </Table.Td>
                      );
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

            <Table.Tr>
              <Table.Td />
              <Table.Td>
                <Text className={classes.summaryLabelSolved}>Сдали</Text>
              </Table.Td>
              <Table.Td />
              {!byPoints && <Table.Td />}
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
              {!byPoints && <Table.Td />}
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
