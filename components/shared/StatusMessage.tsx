"use client";

import { Card, Group, Text } from "@mantine/core";
import { IconCheck, IconX } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import classes from "./StatusMessage.module.css";

interface StatusMessageProps {
  type: "success" | "error";
  message: string;
  opened: boolean;
  onClose: () => void;
}

export function StatusMessage({
  type,
  message,
  opened,
  onClose,
}: StatusMessageProps) {
  const [isClosing, setIsClosing] = useState(false);

  useEffect(() => {
    if (opened) {
      setIsClosing(false);
      const timer = setTimeout(() => {
        setIsClosing(true);
        setTimeout(() => {
          onClose();
        }, 300); // Длительность анимации закрытия
      }, 1200);

      return () => clearTimeout(timer);
    }
  }, [opened, onClose]);

  if (!opened) return null;

  const Icon = type === "success" ? IconCheck : IconX;
  const iconColor = type === "success" ? "var(--mantine-color-green-6)" : "var(--mantine-color-red-6)";

  return (
    <Card
      shadow="md"
      padding="sm"
      radius={0}
      className={isClosing ? classes.slideUp : classes.slideDown}
      style={{
        position: "fixed",
        top: "15%",
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: 1000,
        backgroundColor: "var(--mantine-color-dark-6)",
      }}
    >
      <Group gap="sm" justify="center" wrap="nowrap">
        <Icon size={18} color={iconColor} />
        <Text size="sm" c="white" fw={500}>
          {message}
        </Text>
      </Group>
    </Card>
  );
}

