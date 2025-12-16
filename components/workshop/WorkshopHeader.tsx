"use client";

import { createContest, createProblem } from "@/lib/actions";
import { Button, Group, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import { usePageTransition } from "./WorkshopPageWrapper";
import { useState, useEffect } from "react";
import { flushSync } from "react-dom";

type Props = {
  isAuthenticated: boolean;
};

export function WorkshopHeader({ isAuthenticated }: Props) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const currentView = searchParams.get("view") || "contests";
  const { pendingView } = usePageTransition();
  const [localView, setLocalView] = useState<string>(currentView);
  
  // Sync local view with current view when URL changes
  useEffect(() => {
    setLocalView(currentView);
  }, [currentView]);
  
  // Use pendingView if transition is happening, otherwise use localView
  const view = pendingView || localView;

  const createContestMutation = useMutation({
    mutationFn: async () => {
      const [error, response] = await createContest("New Contest");
      if (error) throw new Error(error.message);
      if (!response?.id) throw new Error("Не получен ID контеста");
      return response.id;
    },
    onSuccess: (contestId: string) => {
      router.push(`/contests/${contestId}`);
    },
    onError: (error) => {
      console.error("Не удалось создать контест:", error);
      notifications.show({
        title: "Ошибка",
        message:
          error instanceof Error ? error.message : "Не удалось создать контест",
        color: "red",
      });
    },
  });

  const createProblemMutation = useMutation({
    mutationFn: async (): Promise<string> => {
      const [error, response] = await createProblem("New Problem");
      if (error) throw new Error(error.message);
      if (!response?.id) throw new Error("Не получен ID задачи");
      return response.id;
    },
    onSuccess: (problemId: string) => {
      router.push(`/problems/${problemId}/edit`);
    },
    onError: (error) => {
      console.error("Не удалось создать задачу:", error);
      notifications.show({
        title: "Ошибка",
        message:
          error instanceof Error ? error.message : "Не удалось создать задачу",
        color: "red",
      });
    },
  });

  const handleCreate = () => {
    if (view === "problems") {
      createProblemMutation.mutate();
    } else {
      createContestMutation.mutate();
    }
  };

  const isLoading = createContestMutation.isPending || createProblemMutation.isPending;
  const buttonText = view === "problems" ? "Создать задачу" : "Создать контест";
  const buttonTitle = view === "problems" ? "Создать новую задачу" : "Создать новый контест";

  return (
    <Group justify="space-between" align="flex-end">
      <Title order={2}>Мастерская</Title>
      {isAuthenticated && (
        <Button
          title={buttonTitle}
          onClick={handleCreate}
          loading={isLoading}
          size="md"
          leftSection={<IconPlus size={18} />}
          radius="md"
        >
          {buttonText}
        </Button>
      )}
    </Group>
  );
}

