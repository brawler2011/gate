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

const RESERVED_CONTEST_LOGINS = new Set([
  "problems",
  "teams",
  "members",
  "settings",
  "submit",
  "mysubmissions",
  "submissions",
  "monitor",
]);

export const CreateContestModal = ({
  opened,
  onClose,
  orgs,
  defaultOrgId,
  lockOrganization = false,
}: Props): ReactNode => {
  const router = useRouter();
  const [title, setTitle] = useState("New Contest");
  const [login, setLogin] = useState("");
  const [orgId, setOrgId] = useState<string | null>(defaultOrgId ?? null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (opened) {
      setTitle("New Contest");
      setLogin("");
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

    const trimmedLogin = login.trim();
    if (trimmedLogin && RESERVED_CONTEST_LOGINS.has(trimmedLogin)) {
      notifications.show({
        title: "Ошибка",
        message: `Логин '${trimmedLogin}' зарезервирован`,
        color: "red",
      });
      return;
    }

    setLoading(true);
    try {
      const selectedOrg = orgs.find((o) => o.id === orgId);
      if (!selectedOrg) {
        return;
      }
      const [error, response] = await api.createContest({
        orgLogin: selectedOrg.login,
        title: title.trim() || "New Contest",
        login: trimmedLogin || undefined,
      });
      if (error) {
        throw new Error(error.message);
      }
      const orgSlug = selectedOrg.login;
      onClose();
      router.push(`/${orgSlug}/${response.login}`);
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
        <TextInput
          label="Логин / URL (необязательно)"
          placeholder="Оставьте пустым для автогенерации"
          value={login}
          onChange={(e) => setLogin(e.currentTarget.value)}
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
