"use client";

import { createProblem, getProblems } from "@/lib/actions";
import type { OrganizationModel } from "@contracts/core/v1";
import {
  Button,
  Modal,
  Select,
  Stack,
  TextInput,
} from "@mantine/core";
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

export function CreateProblemModal({
  opened,
  onClose,
  orgs,
  defaultOrgId,
  lockOrganization = false,
}: Props) {
  const router = useRouter();
  const [title, setTitle] = useState("New Problem");
  const [orgId, setOrgId] = useState<string | null>(defaultOrgId ?? null);
  const [templateId, setTemplateId] = useState<string | null>(null);
  const [templates, setTemplates] = useState<Array<{ value: string; label: string }>>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (opened) {
      setTitle("New Problem");
      const orgIds = new Set(orgs.map((o) => o.id));
      const validDefault =
        defaultOrgId && orgIds.has(defaultOrgId) ? defaultOrgId : null;
      setOrgId(validDefault ?? (orgs.length === 1 ? orgs[0].id : null));
      setTemplateId(null);
    }
  }, [opened, defaultOrgId, orgs]);

  useEffect(() => {
    if (opened && orgId) {
      setLoadingTemplates(true);
      getProblems(1, 100, undefined, undefined, undefined, orgId, true)
        .then(([err, res]) => {
          if (!err && res?.problems) {
            setTemplates(
              res.problems.map((p) => ({
                value: p.id,
                label: p.title,
              }))
            );
          } else {
            setTemplates([]);
          }
        })
        .catch(() => {
          setTemplates([]);
        })
        .finally(() => {
          setLoadingTemplates(false);
        });
    } else {
      setTemplates([]);
      setTemplateId(null);
    }
  }, [opened, orgId]);

  const orgData = orgs.map((o) => ({ value: o.id, label: o.name }));

  const handleSubmit = async () => {
    if (!orgId) return;

    setLoading(true);
    try {
      const [createError, createResponse] = await createProblem(
        title.trim() || "New Problem",
        orgId,
        templateId ?? undefined,
      );
      if (createError) throw new Error(createError.message);
      if (!createResponse?.id) throw new Error("Не получен ID задачи");

      const problemId = createResponse.id;

      onClose();
      router.push(`/problems/${problemId}`);
    } catch (err) {
      notifications.show({
        title: "Ошибка",
        message:
          err instanceof Error ? err.message : "Не удалось создать задачу",
        color: "red",
      });
    } finally {
      setLoading(false);
    }
  };

  const isValid = title.trim() && orgId;

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
        <Select
          label="Шаблон задачи"
          placeholder={loadingTemplates ? "Загрузка шаблонов..." : "Выберите шаблон (необязательно)"}
          data={templates}
          value={templateId}
          onChange={setTemplateId}
          clearable
          disabled={loadingTemplates || templates.length === 0}
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
