"use client";

import {Badge, Box, Group, Switch, Table, Text, TextInput, Title, Tooltip} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconSearch} from "@tabler/icons-react";
import Link from "next/link";
import React, {useEffect, useState, useMemo} from "react";

import {api} from "@/lib/api";
import {env} from "@/lib/env";
import {numberToLetters} from "@/lib/lib";
import {submissionsWsManager} from "@/lib/submissionsWsManager";

import classes from "./ContestMonitorTable.module.css";

import type {
  ScoreboardResponseModel,
  ScoreboardItemModel,
  ScoreboardProblemResultModel,
  ScoreboardProblemHeaderModel,
} from "@/contracts/core/v1";
import type {SubmissionsMessage, MessageSubmissionCompleted} from "@/contracts/observer/v1";

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

const getProblemLetter = (ordinal: number, shortName?: string): string => {
  if (shortName && shortName.length <= 3) {
    return shortName;
  }
  return String.fromCharCode(65 + (ordinal - 1));
};

export const ContestMonitorTable = ({
  contestId,
  orgLogin,
  contestLogin,
  initialScoreboard,
  startTime,
  endTime,
  isManager = false,
}: ContestMonitorTableProps): React.ReactNode => {
  const [searchQuery, setSearchQuery] = useState("");
  const [items, setItems] = useState<ScoreboardItemModel[]>(initialScoreboard.items || []);
  const [problems] = useState<ScoreboardProblemHeaderModel[]>(initialScoreboard.problems || []);
  const [isFrozen, setIsFrozen] = useState<boolean>(initialScoreboard.is_frozen ?? false);
  const [showRealMonitor, setShowRealMonitor] = useState<boolean>(false);
  const [loadingScoreboard, setLoadingScoreboard] = useState<boolean>(false);

  const penaltyPerAttempt = initialScoreboard.penalty_per_attempt || 20;

  const handleToggleRealMonitor = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const checked = e.currentTarget.checked;
    setShowRealMonitor(checked);
    setLoadingScoreboard(true);
    const [err, res] = await api.getContestScoreboard({
      orgLogin,
      contestLogin,
      unfrozen: checked,
    });
    setLoadingScoreboard(false);
    if (err) {
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось загрузить таблицу",
        color: "red",
      });
      return;
    }
    if (res) {
      setItems(res.items || []);
      if (!checked) {
        setIsFrozen(res.is_frozen ?? false);
      }
    }
  };

  useEffect(() => {
    const wsUrl = `${env.getWebSocketUrl()}/ws/submissions`;

    const handleMessage = (event: MessageEvent) => {
      try {
        const msg: SubmissionsMessage = JSON.parse(event.data);
        if (msg.event_type !== "submissions.completed" || !msg.payload) {
          return;
        }

        const payload = msg.payload as MessageSubmissionCompleted;
        if (
          payload.contest_id !== contestId ||
          !payload.user_id ||
          !payload.problem_id ||
          payload.state === undefined
        ) {
          return;
        }

        if (payload.state === 1 || payload.state === 101) {
          return;
        }

        if (payload.created_at) {
          const createdAtTime = new Date(payload.created_at).getTime();
          if (startTime && createdAtTime < new Date(startTime).getTime()) {
            return;
          }
          if (endTime && createdAtTime > new Date(endTime).getTime()) {
            return;
          }
        }

        if (isFrozen && !showRealMonitor) {
          setItems((prevItems) => {
            const userId = payload.user_id;
            const problemId = payload.problem_id;

            const existingItemIndex = prevItems.findIndex((item) => item.user_id === userId);
            if (existingItemIndex === -1) {
              return prevItems;
            }

            const existingItem = prevItems[existingItemIndex];
            const pResults = [...existingItem.problem_results];
            const pIndex = pResults.findIndex((r) => r.problem_id === problemId);

            const pResult: ScoreboardProblemResultModel =
              pIndex !== -1
                ? {...pResults[pIndex]}
                : {
                  problem_id: problemId!,
                  solved: false,
                  failed_attempts: 0,
                  pending_attempts: 0,
                };

            pResult.pending_attempts = (pResult.pending_attempts || 0) + 1;

            if (pIndex !== -1) {
              pResults[pIndex] = pResult;
            } else {
              pResults.push(pResult);
            }

            const updatedItem: ScoreboardItemModel = {
              ...existingItem,
              problem_results: pResults,
            };

            const newItems = [...prevItems];
            newItems[existingItemIndex] = updatedItem;
            return newItems;
          });
          return;
        }

        setItems((prevItems) => {
          const userId = payload.user_id;
          const problemId = payload.problem_id;
          const state = payload.state;
          const createdAtStr = payload.created_at || new Date().toISOString();

          const existingItemIndex = prevItems.findIndex((item) => item.user_id === userId);
          if (existingItemIndex === -1) {
            return prevItems;
          }

          const existingItem = prevItems[existingItemIndex];
          const pResults = [...existingItem.problem_results];
          const pIndex = pResults.findIndex((r) => r.problem_id === problemId);

          const pResult: ScoreboardProblemResultModel = pIndex !== -1
            ? {...pResults[pIndex]}
            : {
              problem_id: problemId!,
              solved: false,
              failed_attempts: 0,
              pending_attempts: 0,
            };

          if (pResult.solved) {
            return prevItems;
          }

          if (state === 200) {
            pResult.solved = true;
            pResult.first_ac_time = createdAtStr;
            if (startTime) {
              const mins = Math.max(
                0,
                Math.floor((new Date(createdAtStr).getTime() - new Date(startTime).getTime()) / 60000)
              );
              pResult.time_minutes = mins;
            } else {
              pResult.time_minutes = 0;
            }
            pResult.penalty = pResult.failed_attempts * penaltyPerAttempt;
          } else if ([102, 103, 104, 105, 106].includes(state)) {
            pResult.failed_attempts += 1;
          } else {
            return prevItems;
          }

          if (pIndex !== -1) {
            pResults[pIndex] = pResult;
          } else {
            pResults.push(pResult);
          }

          let solvedCount = 0;
          let totalPenalty = 0;
          let lastAC: string | undefined = undefined;

          for (const r of pResults) {
            if (r.solved) {
              solvedCount += 1;
              totalPenalty += (r.time_minutes || 0) + (r.failed_attempts * penaltyPerAttempt);
              if (r.first_ac_time) {
                if (!lastAC || new Date(r.first_ac_time).getTime() > new Date(lastAC).getTime()) {
                  lastAC = r.first_ac_time;
                }
              }
            }
          }

          const updatedItem: ScoreboardItemModel = {
            ...existingItem,
            problems_solved: solvedCount,
            total_penalty: totalPenalty,
            last_accepted_at: lastAC,
            problem_results: pResults,
          };

          const newItems = [...prevItems];
          newItems[existingItemIndex] = updatedItem;

          newItems.sort((a, b) => {
            if (a.problems_solved !== b.problems_solved) {
              return b.problems_solved - a.problems_solved;
            }
            if (a.total_penalty !== b.total_penalty) {
              return a.total_penalty - b.total_penalty;
            }
            if (a.last_accepted_at && b.last_accepted_at) {
              const diff = new Date(a.last_accepted_at).getTime() - new Date(b.last_accepted_at).getTime();
              if (diff !== 0) {
                return diff;
              }
            } else if (a.last_accepted_at) {
              return -1;
            } else if (b.last_accepted_at) {
              return 1;
            }
            return a.username.localeCompare(b.username);
          });

          return newItems;
        });
      } catch {
        // ignore parse error
      }
    };

    const listenerId = submissionsWsManager.addListener({
      key: `monitor-${contestId}`,
      url: wsUrl,
      enabled: true,
      onMessage: handleMessage,
      onConnectionError: () => {},
      onFatalConnectionError: () => {},
      onStatusChange: () => {},
      onResyncRequired: async () => {},
    });

    return () => {
      submissionsWsManager.removeListener(listenerId);
    };
  }, [contestId, startTime, endTime, penaltyPerAttempt, isFrozen, showRealMonitor]);

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
              onChange={handleToggleRealMonitor}
              disabled={loadingScoreboard}
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
                        href={`/${orgLogin}/contests/${contestLogin}/problems/${letter}`}
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
