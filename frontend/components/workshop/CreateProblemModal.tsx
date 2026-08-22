"use client";

import {
  Button,
  Modal,
  Select,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useRouter} from "next/navigation";
import {useEffect, useState} from "react";

import {api} from "@/lib/api";

import type {OrganizationModel, ProblemTemplateModel} from "@/contracts/core/v1";
import type {ComboboxItem, ComboboxItemGroup} from "@mantine/core";
import type {ReactNode} from "react";

type Props = {
  opened: boolean;
  onClose: () => void;
  orgs: OrganizationModel[];
  defaultOrgId?: string;
  lockOrganization?: boolean;
};

export const CreateProblemModal = ({
  opened,
  onClose,
  orgs,
  defaultOrgId,
  lockOrganization = false,
}: Props): ReactNode => {
  const router = useRouter();
  const [title, setTitle] = useState("New Problem");
  const [orgId, setOrgId] = useState<string | null>(defaultOrgId ?? null);
  const [templateId, setTemplateId] = useState<string | null>("builtin:a-plus-b");
  const [templateList, setTemplateList] = useState<ProblemTemplateModel[]>([]);
  const [templateSelectData, setTemplateSelectData] = useState<Array<ComboboxItemGroup<ComboboxItem>>>([]);
  const [loadingTemplates, setLoadingTemplates] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (opened) {
      setTitle("New Problem");
      const orgIds = new Set(orgs.map((o) => o.id));
      const validDefault =
        defaultOrgId && orgIds.has(defaultOrgId) ? defaultOrgId : null;
      setOrgId(validDefault ?? (orgs.length === 1 ? orgs[0].id : null));
      setTemplateId("builtin:a-plus-b");
    }
  }, [opened, defaultOrgId, orgs]);

  useEffect(() => {
    if (opened && orgId) {
      setLoadingTemplates(true);
      api.listProblemTemplates({organizationId: orgId})
        .then(([err, res]) => {
          if (!err && res) {
            setTemplateList(res);

            const builtinItems: ComboboxItem[] = res
              .filter((t) => t.is_builtin)
              .map((t) => ({value: t.id, label: t.title}));
            const orgItems: ComboboxItem[] = res
              .filter((t) => !t.is_builtin)
              .map((t) => ({value: t.id, label: t.title}));

            const groups: Array<ComboboxItemGroup<ComboboxItem>> = [];
            if (builtinItems.length > 0) {
              groups.push({
                group: "Системные шаблоны",
                items: builtinItems,
              });
            }
            if (orgItems.length > 0) {
              groups.push({
                group: "Шаблоны организации",
                items: orgItems,
              });
            }

            setTemplateSelectData(groups);
            setTemplateId((prev) => {
              if (prev && res.some((t) => t.id === prev)) {
                return prev;
              }
              return res[0]?.id ?? "builtin:a-plus-b";
            });
          } else {
            setTemplateList([]);
            setTemplateSelectData([]);
          }
        })
        .catch(() => {
          setTemplateList([]);
          setTemplateSelectData([]);
        })
        .finally(() => {
          setLoadingTemplates(false);
        });
    } else {
      setTemplateList([]);
      setTemplateSelectData([]);
    }
  }, [opened, orgId]);

  const orgData = orgs.map((o) => ({value: o.id, label: o.name}));
  const selectedTemplate = templateList.find((t) => t.id === templateId);

  const handleSubmit = async () => {
    if (!orgId || !templateId) {
      return;
    }

    setLoading(true);
    try {
      const [createError, createResponse] = await api.createProblem({
        title: title.trim() || "New Problem",
        organizationId: orgId,
        templateId: templateId,
      });
      if (createError) {
        throw new Error(createError.message);
      }
      if (!createResponse?.id) {
        throw new Error("Не получен ID задачи");
      }

      const problemId = createResponse.id;
      const selectedOrg = orgs.find((o) => o.id === orgId);
      const orgSlug = selectedOrg?.login || selectedOrg?.id || "org";

      onClose();
      router.push(`/${orgSlug}/problems/${problemId}`);
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

  const isValid = Boolean(title.trim() && orgId && templateId);

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Новая задача"
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
        <div>
          <Select
            label="Шаблон задачи"
            placeholder={loadingTemplates ? "Загрузка шаблонов..." : "Выберите шаблон"}
            data={templateSelectData}
            value={templateId}
            onChange={setTemplateId}
            required
            allowDeselect={false}
            disabled={loadingTemplates || templateSelectData.length === 0}
          />
          {selectedTemplate?.description && (
            <Text size="xs" c="dimmed" mt={4}>
              {selectedTemplate.description}
            </Text>
          )}
        </div>
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
};
