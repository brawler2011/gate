"use client";

import { Box, Button, Collapse, Stack } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconChevronDown,
  IconChevronUp,
  IconDeviceDesktop,
  IconMail,
  IconMenu2,
  IconPuzzle,
  IconSend,
  IconUser,
} from "@tabler/icons-react";
import Link from "next/link";
import type { ContestModel } from "@contracts/core/v1";
import { CONTEST_CONTENT_MAX_WIDTH } from "@/lib/constants";
import { PermissionChecker } from "@/lib/permissions";
import type { SessionUser } from "@/lib/auth";
import type { ContestRole } from "@/lib/contest-role";
import classes from "./styles.module.css";

type ContestHotbarProps = {
  contest: ContestModel;
  user: SessionUser;
  contestRole: { role: ContestRole } | null;
  activeTab?: "tasks" | "submit" | "submissions" | "monitor" | "manage" | "mysubmissions" | "allsubmissions";
  children?: React.ReactNode;
  maxWidth?: string | number;
  align?: "center" | "left";
};

export function ContestHotbar({ contest, user, contestRole, activeTab, children, maxWidth, align = "center" }: ContestHotbarProps) {
  // Create permission checker
  const checker = new PermissionChecker(user, contestRole?.role || null);
  const [mobileNavOpened, { toggle: toggleMobileNav }] = useDisclosure(false);

  const marginStyle = align === "center" ? "0 auto" : "0";

  // Build tabs array based on permissions
  const tabs = [
    checker.canViewProblems(contest) && {
      key: "tasks",
      label: "Задачи",
      href: `/contests/${contest.id}`,
      icon: <IconPuzzle size={16} />,
    },
    checker.canSubmitSolution(contest) && {
      key: "submit",
      label: "Послать решение",
      href: `/contests/${contest.id}/submit`,
      icon: <IconSend size={16} />,
    },
    checker.canViewMySubmissions(contest) && {
      key: "mysubmissions",
      label: "Мои посылки",
      href: `/contests/${contest.id}/mysubmissions?sortOrder=desc&userId=${user?.id}`,
      icon: <IconUser size={16} />,
    },
    checker.canViewAllSubmissions(contest) && {
      key: "allsubmissions",
      label: "Все посылки",
      href: `/contests/${contest.id}/submissions?sortOrder=desc`,
      icon: <IconMail size={16} />,
    },
    checker.canViewMonitor(contest) && {
      key: "monitor",
      label: "Монитор",
      href: `/contests/${contest.id}/monitor`,
      icon: <IconDeviceDesktop size={16} />,
    },
  ].filter(Boolean) as Array<{ key: string; label: string; href: string; icon: React.ReactNode }>;

  return (
    <Box>
      {/* Desktop tabs - just tabs, no panel */}
      <div className={classes.desktopTabs}>
        <Box style={{ maxWidth: CONTEST_CONTENT_MAX_WIDTH, margin: marginStyle, marginBottom: -1, position: "relative", zIndex: 1 }}>
          <div className={classes.tabRow}>
            {tabs.map((tab) => (
              <Link
                key={tab.key}
                href={tab.href}
                className={`${classes.tab} ${activeTab === tab.key ? classes.tabActive : ""}`}
              >
                {tab.icon}
                {tab.label}
              </Link>
            ))}
          </div>
        </Box>
        <Box style={{ maxWidth: maxWidth || CONTEST_CONTENT_MAX_WIDTH, margin: marginStyle }}>
          {children}
        </Box>
      </div>

      {/* Mobile navigation - just nav, no panel wrapper */}
      <Stack
        gap="md"
        className={classes.mobileSection}
        style={{ maxWidth: maxWidth || CONTEST_CONTENT_MAX_WIDTH, margin: marginStyle }}
      >
        <Button
          onClick={toggleMobileNav}
          variant="default"
          size="md"
          fullWidth
          leftSection={<IconMenu2 size={18} />}
          rightSection={mobileNavOpened ? <IconChevronUp size={18} /> : <IconChevronDown size={18} />}
        >
          Навигация
        </Button>
        
        <Collapse in={mobileNavOpened}>
          <Stack gap="xs">
            {checker.canViewProblems(contest) && (
              <Button
                component={Link}
                href={`/contests/${contest.id}`}
                variant={activeTab === "tasks" ? "filled" : "light"}
                size="md"
                leftSection={<IconPuzzle size={18} />}
                fullWidth
              >
                Задачи
              </Button>
            )}
            {checker.canSubmitSolution(contest) && (
              <Button
                component={Link}
                href={`/contests/${contest.id}/submit`}
                variant={activeTab === "submit" ? "filled" : "light"}
                size="md"
                leftSection={<IconSend size={18} />}
                fullWidth
              >
                Послать решение
              </Button>
            )}
            {checker.canViewMySubmissions(contest) && (
              <Button
                component={Link}
                href={`/contests/${contest.id}/mysubmissions?sortOrder=desc&userId=${user?.id}`}
                variant={activeTab === "mysubmissions" ? "filled" : "light"}
                size="md"
                leftSection={<IconUser size={18} />}
                fullWidth
              >
                Мои посылки
              </Button>
            )}
            {checker.canViewAllSubmissions(contest) && (
              <Button
                component={Link}
                href={`/contests/${contest.id}/submissions?sortOrder=desc`}
                variant={activeTab === "allsubmissions" ? "filled" : "light"}
                size="md"
                leftSection={<IconMail size={18} />}
                fullWidth
              >
                Все посылки
              </Button>
            )}
            {checker.canViewMonitor(contest) && (
              <Button
                component={Link}
                href={`/contests/${contest.id}/monitor`}
                variant={activeTab === "monitor" ? "filled" : "light"}
                size="md"
                leftSection={<IconDeviceDesktop size={18} />}
                fullWidth
              >
                Монитор
              </Button>
            )}
          </Stack>
        </Collapse>
        
        {children}
      </Stack>
    </Box>
  );
}
