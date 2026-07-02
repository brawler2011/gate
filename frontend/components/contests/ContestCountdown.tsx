"use client";

import { useEffect, useState } from "react";
import { Paper, Text, Group, Stack, Title } from "@mantine/core";
import { APP_COLORS } from "@/lib/theme/colors";

interface ContestCountdownProps {
  startTime: string;
  title: string;
}

export function ContestCountdown({ startTime, title }: ContestCountdownProps) {
  const [timeLeft, setTimeLeft] = useState<{
    days: number;
    hours: number;
    minutes: number;
    seconds: number;
    total: number;
  } | null>(null);

  useEffect(() => {
    const calculateTimeLeft = () => {
      const difference = new Date(startTime).getTime() - new Date().getTime();
      if (difference <= 0) {
        return { days: 0, hours: 0, minutes: 0, seconds: 0, total: 0 };
      }

      return {
        days: Math.floor(difference / (1000 * 60 * 60 * 24)),
        hours: Math.floor((difference / (1000 * 60 * 60)) % 24),
        minutes: Math.floor((difference / 1000 / 60) % 60),
        seconds: Math.floor((difference / 1000) % 60),
        total: difference,
      };
    };

    setTimeLeft(calculateTimeLeft());

    const timer = setInterval(() => {
      const left = calculateTimeLeft();
      setTimeLeft(left);
      if (left.total <= 0) {
        clearInterval(timer);
        window.location.reload();
      }
    }, 1000);

    return () => clearInterval(timer);
  }, [startTime]);

  if (!timeLeft) return null;

  const pad = (num: number) => num.toString().padStart(2, "0");

  const formattedStart = new Date(startTime).toLocaleString("ru-RU", {
    dateStyle: "long",
    timeStyle: "short",
  });

  return (
    <Paper
      shadow="md"
      radius="lg"
      p="xl"
      withBorder
      style={{
        background: "rgba(255, 255, 255, 0.02)",
        backdropFilter: "blur(8px)",
        border: "1px solid var(--mantine-color-default-border)",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "300px",
      }}
    >
      <Stack gap="lg" align="center" style={{ width: "100%" }}>
        <Title order={2} c="dimmed" size="h3" fw={600} ta="center">
          {title}
        </Title>
        <Text size="lg" fw={500} ta="center" c={APP_COLORS.contests || "blue"}>
          До начала соревнования осталось:
        </Text>

        <Group gap="xs" justify="center" wrap="nowrap">
          {timeLeft.days > 0 && (
            <>
              <TimeBlock value={pad(timeLeft.days)} label="дн." />
              <Colon />
            </>
          )}
          <TimeBlock value={pad(timeLeft.hours)} label="час." />
          <Colon />
          <TimeBlock value={pad(timeLeft.minutes)} label="мин." />
          <Colon />
          <TimeBlock value={pad(timeLeft.seconds)} label="сек." />
        </Group>

        <Text size="sm" c="dimmed" ta="center">
          Начало: {formattedStart} (по местному времени)
        </Text>
      </Stack>
    </Paper>
  );
}

interface TimeBlockProps {
  value: string;
  label: string;
}

function TimeBlock({ value, label }: TimeBlockProps) {
  return (
    <Stack gap={4} align="center" style={{ minWidth: "60px" }}>
      <Paper
        shadow="xs"
        radius="md"
        p="xs"
        withBorder
        style={{
          width: "100%",
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          background: "var(--mantine-color-body)",
        }}
      >
        <Text size="2rem" fw={700} style={{ fontFamily: "monospace", lineHeight: 1 }}>
          {value}
        </Text>
      </Paper>
      <Text size="xs" c="dimmed" fw={600} tt="uppercase">
        {label}
      </Text>
    </Stack>
  );
}

function Colon() {
  return (
    <Text size="2rem" fw={700} c="dimmed" style={{ lineHeight: 1, marginTop: "-20px" }}>
      :
    </Text>
  );
}
