"use client";

import {Alert, Button, Modal, Stack, TextInput} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconAlertCircle, IconPlus} from "@tabler/icons-react";
import {useRouter} from "next/navigation";
import {useState} from "react";

import {api} from "@/lib/api";

import type {ReactNode} from "react";

type Props = {
  orgId: string;
};

export const CreateTeamModal = ({orgId}: Props): ReactNode => {
  const [opened, {open, close}] = useDisclosure(false);
  const [name, setName] = useState("");
  const [loading, setLoading] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const router = useRouter();

  const handleClose = () => {
    close();
    setName("");
    setNameError(null);
    setFormError(null);
  };

  const handleCreate = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      return;
    }

    if (trimmed.length < 2) {
      setNameError("Название должно содержать не менее 2 символов");
      return;
    }

    setLoading(true);
    const [error, response] = await api.createTeam({
      requestBody: {
        name: trimmed,
        organization_id: orgId,
      },
    });
    setLoading(false);

    if (error) {
      setFormError(error.message || "Не удалось создать команду");
      return;
    }

    notifications.show({
      title: "Успешно",
      message: "Команда успешно создана",
      color: "green",
    });

    handleClose();
    router.refresh();
  };

  return (
    <>
      <Button
        size="md"
        radius="md"
        leftSection={<IconPlus size={18} />}
        onClick={open}
      >
        Создать команду
      </Button>
      <Modal opened={opened} onClose={handleClose} title="Новая команда" size="xs" centered>
        <Stack gap="sm">
          {formError && (
            <Alert
              color="red"
              variant="light"
              title="Ошибка"
              icon={<IconAlertCircle size={16} />}
            >
              {formError}
            </Alert>
          )}
          <TextInput
            label="Название команды"
            placeholder="Моя команда"
            value={name}
            error={nameError}
            onChange={(e) => {
              setName(e.currentTarget.value);
              setNameError(null);
              setFormError(null);
            }}
            onKeyDown={(e) => e.key === "Enter" && handleCreate()}
            autoFocus
          />
          <Button loading={loading} onClick={handleCreate} disabled={!name.trim()}>
            Создать
          </Button>
        </Stack>
      </Modal>
    </>
  );
};
