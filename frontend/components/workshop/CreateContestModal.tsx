"use client";

import { createContest } from "@/lib/actions";
import type { OrganizationModel } from "@contracts/core/v1";
import { Button, Modal, Select, Stack, TextInput } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

type Props = {
  opened: boolean;
  onClose: () => void;
  orgs: OrganizationModel[];
  defaultOrgId?: string;
  lockOrganization?: boolean;
};

export function CreateContestModal({
  opened,
  onClose,
  orgs,
  defaultOrgId,
  lockOrganization = false,
}: Props) {
  const router = useRouter();
  const [title, setTitle] = useState("New Contest");
  const [orgId, setOrgId] = useState<string | null>(defaultOrgId ?? null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (opened) {
      setTitle("New Contest");
      const orgIds = new Set(orgs.map((o) => o.id));
      const validDefault =
        defaultOrgId && orgIds.has(defaultOrgId) ? defaultOrgId : null;
      setOrgId(validDefault ?? (orgs.length === 1 ? orgs[0].id : null));
    }
  }, [opened, defaultOrgId, orgs]);

  const orgData = orgs.map((o) => ({ value: o.id, label: o.name }));

  const handleSubmit = async () => {
    if (!orgId) return;

    setLoading(true);
    try {
      const [error, response] = await createContest(
        title.trim() || "New Contest",
        orgId,
      );
      if (error) throw new Error(error.message);
      if (!response?.id) throw new Error("Не получен ID контеста");
      onClose();
      router.push(`/contests/${response.id}`);
    } catch (err) {
      notifications.show({
        title: "Ошибка",
        message:
          err instanceof Error ? err.message : "Не удалось создать контест",
        color: "red",
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Новый контест"
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
        {!lockOrganization && (
          <Select
            label="Организация"
            placeholder="Выберите организацию"
            data={orgData}
            value={orgId}
            onChange={setOrgId}
            required
          />
        )}
        <Button
          onClick={handleSubmit}
          loading={loading}
          disabled={!title.trim() || !orgId}
          fullWidth
        >
          Создать
        </Button>
      </Stack>
    </Modal>
  );
}
