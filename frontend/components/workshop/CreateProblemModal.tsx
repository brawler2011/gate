"use client";

import { createProblem, updateProblem } from "@/lib/actions";
import { Button, Modal, NumberInput, Select, Stack, TextInput } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import type { OrganizationModel } from "@contracts/gateway/v1";

type Props = {
  opened: boolean;
  onClose: () => void;
  orgs: OrganizationModel[];
  defaultOrgId?: string;
};

export function CreateProblemModal({ opened, onClose, orgs, defaultOrgId }: Props) {
  const router = useRouter();
  const [title, setTitle] = useState("New Problem");
  const [orgId, setOrgId] = useState<string | null>(defaultOrgId ?? null);
  const [timeLimit, setTimeLimit] = useState<number | string>(1000);
  const [memoryLimit, setMemoryLimit] = useState<number | string>(256);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (opened) {
      setTitle("New Problem");
      const orgIds = new Set(orgs.map((o) => o.id));
      const validDefault = defaultOrgId && orgIds.has(defaultOrgId) ? defaultOrgId : null;
      setOrgId(validDefault ?? (orgs.length === 1 ? orgs[0].id : null));
      setTimeLimit(1000);
      setMemoryLimit(256);
    }
  }, [opened, defaultOrgId, orgs]);

  const orgData = orgs.map((o) => ({ value: o.id, label: o.name }));

  const handleSubmit = async () => {
    if (!orgId) return;

    const tl = typeof timeLimit === "number" ? timeLimit : parseInt(timeLimit, 10);
    const ml = typeof memoryLimit === "number" ? memoryLimit : parseInt(memoryLimit, 10);

    if (!tl || !ml) return;

    setLoading(true);
    try {
      const [createError, createResponse] = await createProblem(title.trim() || "New Problem", orgId);
      if (createError) throw new Error(createError.message);
      if (!createResponse?.id) throw new Error("Не получен ID задачи");

      const problemId = createResponse.id;

      const [updateError] = await updateProblem(problemId, {
        time_limit: tl,
        memory_limit: ml,
      });
      if (updateError) throw new Error(updateError.message);

      onClose();
      router.push(`/problems/${problemId}`);
    } catch (err) {
      notifications.show({
        title: "Ошибка",
        message: err instanceof Error ? err.message : "Не удалось создать задачу",
        color: "red",
      });
    } finally {
      setLoading(false);
    }
  };

  const isValid = title.trim() && orgId && timeLimit && memoryLimit;

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Новая задача"
      centered
      size="md"
      radius="md"
      overlayProps={{ backgroundOpacity: 0.4 }}
    >
      <Stack gap="md">
        <TextInput
          label="Название"
          value={title}
          onChange={(e) => setTitle(e.currentTarget.value)}
          onFocus={(e) => e.currentTarget.select()}
          required
          data-autofocus
        />
        <Select
          label="Организация"
          placeholder="Выберите организацию"
          data={orgData}
          value={orgId}
          onChange={setOrgId}
          required
        />
        <NumberInput
          label="Лимит времени (мс)"
          value={timeLimit}
          onChange={setTimeLimit}
          min={100}
          max={30000}
          step={100}
          required
        />
        <NumberInput
          label="Лимит памяти (МБ)"
          value={memoryLimit}
          onChange={setMemoryLimit}
          min={16}
          max={4096}
          step={64}
          required
        />
        <Button
          onClick={handleSubmit}
          loading={loading}
          disabled={!isValid}
          fullWidth
        >
          Создать
        </Button>
      </Stack>
    </Modal>
  );
}
