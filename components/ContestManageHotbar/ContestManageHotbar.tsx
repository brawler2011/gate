"use client";

import { Box, Button, Collapse, Stack } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";
import {
  IconArrowLeft,
  IconChevronDown,
  IconChevronUp,
  IconMenu2,
  IconPuzzle,
  IconSettings,
  IconUsers,
} from "@tabler/icons-react";
import Link from "next/link";
import { CONTEST_CONTENT_MAX_WIDTH } from "@/lib/constants";
import classes from "./styles.module.css";

type Section = "settings" | "problems" | "participants";

type ContestManageHotbarProps = {
  contestId: string;
  contestTitle: string;
  activeSection: Section;
  children?: React.ReactNode;
  maxWidth?: string | number;
};

// Navigation configuration
const NAV_SECTIONS = [
  {
    key: "settings" as Section,
    label: "Настройки",
    icon: <IconSettings size={16} />,
  },
  {
    key: "problems" as Section,
    label: "Задачи",
    icon: <IconPuzzle size={16} />,
  },
  {
    key: "participants" as Section,
    label: "Участники",
    icon: <IconUsers size={16} />,
  },
] as const;

export function ContestManageHotbar({
  contestId,
  contestTitle,
  activeSection,
  children,
  maxWidth,
}: ContestManageHotbarProps) {
  const [mobileNavOpened, { toggle: toggleMobileNav }] = useDisclosure(false);

  return (
    <Box>
      {/* Desktop tabs with go back button */}
      <div className={classes.desktopTabs}>
        <Box
          style={{
            maxWidth: CONTEST_CONTENT_MAX_WIDTH,
            margin: "0 auto",
            marginBottom: -1,
            position: "relative",
            zIndex: 1,
          }}
        >
          <div className={classes.tabsHeader}>
            {/* Go back button on the left */}
            <Button
              component={Link}
              href={`/contests/${contestId}`}
              variant="subtle"
              size="sm"
              leftSection={<IconArrowLeft size={16} />}
              className={classes.backButton}
            >
              Назад к контесту
            </Button>

            {/* Tabs on the right */}
            <div className={classes.tabRow}>
              {NAV_SECTIONS.map((section) => (
                <Link
                  key={section.key}
                  href={`/contests/${contestId}/manage?section=${section.key}`}
                  className={`${classes.tab} ${
                    activeSection === section.key ? classes.tabActive : ""
                  }`}
                >
                  {section.icon}
                  {section.label}
                </Link>
              ))}
            </div>
          </div>
        </Box>
        <Box
          style={{ maxWidth: maxWidth || CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto" }}
        >
          {children}
        </Box>
      </div>

      {/* Mobile navigation */}
      <Stack
        gap="md"
        className={classes.mobileSection}
        style={{ maxWidth: maxWidth || CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto" }}
      >
        {/* Back button on mobile */}
        <Button
          component={Link}
          href={`/contests/${contestId}`}
          variant="light"
          size="md"
          leftSection={<IconArrowLeft size={18} />}
          fullWidth
        >
          Назад к контесту
        </Button>

        {/* Mobile menu toggle */}
        <Button
          onClick={toggleMobileNav}
          variant="default"
          size="md"
          fullWidth
          leftSection={<IconMenu2 size={18} />}
          rightSection={
            mobileNavOpened ? <IconChevronUp size={18} /> : <IconChevronDown size={18} />
          }
        >
          Навигация
        </Button>

        <Collapse in={mobileNavOpened}>
          <Stack gap="xs">
            {NAV_SECTIONS.map((section) => {
              const Icon = section.icon.type;
              return (
                <Button
                  key={section.key}
                  component={Link}
                  href={`/contests/${contestId}/manage?section=${section.key}`}
                  variant={activeSection === section.key ? "filled" : "light"}
                  size="md"
                  leftSection={<Icon size={18} />}
                  fullWidth
                >
                  {section.label}
                </Button>
              );
            })}
          </Stack>
        </Collapse>

        {children}
      </Stack>
    </Box>
  );
}

