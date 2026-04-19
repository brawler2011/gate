import type { SessionUser } from "@/lib/auth";
import { CONTEST_INFO_PANEL_WIDTH } from "@/lib/constants";
import type { ContestModel } from "@contracts/core/v1";
import { Paper, Stack, Title } from "@mantine/core";
import Link from "next/link";

type ContestInfoPanelProps = {
  contest: ContestModel;
  user: SessionUser;
  width?: string | number;
};

/**
 * Contest info panel component
 * Shows contest name
 * Only visible for authenticated users, hidden on mobile
 */
export function ContestInfoPanel({
  contest,
  user,
  width,
}: ContestInfoPanelProps) {
  // Don't render for unauthenticated users
  if (!user) {
    return null;
  }

  return (
    <Paper
      shadow="none"
      radius="md"
      px="sm"
      py="xs"
      withBorder
      bg="transparent"
      style={{
        width: width || CONTEST_INFO_PANEL_WIDTH,
        borderColor: "var(--mantine-color-dark-5)",
      }}
    >
      <Stack gap="sm" align="center">
        {/* Contest Title */}
        <Link
          href={`/contests/${contest.id}`}
          style={{ textDecoration: "none", width: "100%" }}
        >
          <Title
            order={3}
            lineClamp={2}
            ta="center"
            style={{ fontSize: "1.25rem", cursor: "pointer" }}
            c="var(--mantine-color-text)"
          >
            {contest.title}
          </Title>
        </Link>
      </Stack>
    </Paper>
  );
}
