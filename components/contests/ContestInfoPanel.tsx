import { Badge, Button, Paper, Stack, Title } from "@mantine/core";
import { IconSettings } from "@tabler/icons-react";
import Link from "next/link";
import type { ContestModel } from "@contracts/core/v1";
import type { ContestRole } from "@/lib/contest-role";
import type { SessionUser } from "@/lib/auth";
import { PermissionChecker } from "@/lib/permissions";
import { CONTEST_INFO_PANEL_WIDTH } from "@/lib/constants";
import classes from "./ContestInfoPanel.module.css";

type ContestInfoPanelProps = {
  contest: ContestModel;
  user: SessionUser;
  contestRole: { role: ContestRole } | null;
  width?: string | number;
};

/**
 * Role display configuration for badges
 */
const ROLE_CONFIG: Record<ContestRole | "guest", { label: string; color: string }> = {
  owner: { label: "Владелец", color: "red" },
  moderator: { label: "Модератор", color: "yellow" },
  participant: { label: "Участник", color: "gray" },
  guest: { label: "Гость", color: "blue" },
};

/**
 * Get role display configuration
 */
function getRoleDisplay(role: ContestRole | null): { label: string; color: string } {
  if (!role) {
    return ROLE_CONFIG.guest;
  }
  return ROLE_CONFIG[role] || ROLE_CONFIG.guest;
}

/**
 * Contest info panel component
 * Shows contest name, user's role badge, and management button for moderators/owners
 * Only visible for authenticated users, hidden on mobile
 */
export function ContestInfoPanel({ contest, user, contestRole, width }: ContestInfoPanelProps) {
  // Don't render for unauthenticated users
  if (!user) {
    return null;
  }

  const checker = new PermissionChecker(user, contestRole?.role || null);
  const canManage = checker.canManageContest(contest);
  const roleDisplay = getRoleDisplay(contestRole?.role || null);

  return (
    <Paper
      shadow="none"
      radius="md"
      p="md"
      withBorder
      bg="transparent"
      style={{ 
        width: width || CONTEST_INFO_PANEL_WIDTH,
        borderColor: 'var(--mantine-color-dark-5)'
      }}
    >
      <Stack gap="sm" align="center">
        {/* Contest Title */}
        <Title order={3} lineClamp={2} ta="center" style={{ fontSize: '1.25rem' }}>
          {contest.title}
        </Title>

        {/* Role Badge */}
        <Badge
          variant="filled"
          color={roleDisplay.color}
          size="lg"
          tt="none"
        >
          {roleDisplay.label}
        </Badge>

        {/* Manage Button - only for moderators and owners */}
        {canManage && (
          <Button
            component={Link}
            href={`/contests/${contest.id}/manage`}
            className={classes.manageButton}
            leftSection={<IconSettings size={16} />}
            size="sm"
            mt="xs"
            variant="transparent"
          >
            Управление
          </Button>
        )}
      </Stack>
    </Paper>
  );
}
