"use client";

import {Button, Modal, Select, Stack, TextInput} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useRouter} from "next/navigation";
import {useEffect, useState} from "react";

import {api} from "@/lib/api";

import type {OrganizationModel} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type Props = {
  opened: boolean;
  onClose: () => void;
  orgs: OrganizationModel[];
  defaultOrgId?: string;
  lockOrganization?: boolean;
};

export const CreateContestModal = ({
  opened,
  onClose,
  orgs,
  defaultOrgId,
  lockOrganization = false,
}: Props): ReactNode => {
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

  const orgData = orgs.map((o) => ({value: o.id, label: o.name}));

  const handleSubmit = async () => {
    if (!orgId) {
      return;
    }

    setLoading(true);
    try {
      const [error, response] = await api.createContest({
        title: title.trim() || "New Contest",
        organizationId: orgId,
      });
      if (error) {
        throw new Error(error.message);
      }
      const selectedOrg = orgs.find((o) => o.id === orgId);
      const orgSlug = selectedOrg?.login || selectedOrg?.id || "org";
      onClose();
      router.push(`/${orgSlug}/contests/${response.id}`);
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
      overlayProps={{backgroundOpacity: 0.4}}
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
};
