"use client";

import { Button, Group, Title } from "@mantine/core";
import { IconPlus } from "@tabler/icons-react";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import type { OrganizationModel } from "@contracts/gateway/v1";
import { CreateContestModal } from "./CreateContestModal";
import { CreateProblemModal } from "./CreateProblemModal";
import { usePageTransition } from "./WorkshopPageWrapper";

type Props = {
  isAuthenticated: boolean;
  orgs: OrganizationModel[];
};

export function WorkshopHeader({ isAuthenticated, orgs }: Props) {
  const searchParams = useSearchParams();
  const currentView = searchParams.get("view") || "contests";
  const orgId = searchParams.get("org_id") ?? undefined;
  const { pendingView } = usePageTransition();

  const view = pendingView || currentView;

  const [contestModalOpened, setContestModalOpened] = useState(false);
  const [problemModalOpened, setProblemModalOpened] = useState(false);

  const handleCreate = () => {
    if (view === "problems") {
      setProblemModalOpened(true);
    } else {
      setContestModalOpened(true);
    }
  };

  const buttonText = view === "problems" ? "Создать задачу" : "Создать контест";
  const buttonTitle = view === "problems" ? "Создать новую задачу" : "Создать новый контест";

  return (
    <>
      <Group justify="space-between" align="flex-end">
        <Title order={2}>Мастерская</Title>
        {isAuthenticated && (
          <Button
            title={buttonTitle}
            onClick={handleCreate}
            size="md"
            leftSection={<IconPlus size={18} />}
            radius="md"
          >
            {buttonText}
          </Button>
        )}
      </Group>

      <CreateContestModal
        opened={contestModalOpened}
        onClose={() => setContestModalOpened(false)}
        orgs={orgs}
        defaultOrgId={orgId}
      />
      <CreateProblemModal
        opened={problemModalOpened}
        onClose={() => setProblemModalOpened(false)}
        orgs={orgs}
        defaultOrgId={orgId}
      />
    </>
  );
}
