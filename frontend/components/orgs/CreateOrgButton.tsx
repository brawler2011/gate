"use client";
import { useState } from 'react';
import { Alert, Button, Modal, TextInput, Stack } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { useRouter } from 'next/navigation';
import { createOrganization } from '@/lib/actions';
import { IconAlertCircle, IconPlus } from '@tabler/icons-react';

const translateApiError = (message: string): string => {
  if (message.includes('at least one latin letter or digit')) {
    return 'Название должно содержать хотя бы одну латинскую букву или цифру';
  }
  if (message.includes('between 3 and 64 characters')) {
    return 'Название должно быть от 3 до 64 символов';
  }
  if (message.includes('failed to create organization')) {
    return 'Не удалось создать организацию. Попробуйте позже';
  }
  return message;
};

export function CreateOrgButton() {
  const [opened, { open, close }] = useDisclosure(false);
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const router = useRouter();

  const handleClose = () => {
    close();
    setName('');
    setNameError(null);
    setFormError(null);
  };

  const handleCreate = async () => {
    const trimmed = name.trim();
    if (!trimmed) return;

    if (trimmed.length < 3) {
      setNameError('Название должно содержать не менее 3 символов');
      return;
    }
    if (trimmed.length > 64) {
      setNameError('Название не должно превышать 64 символа');
      return;
    }

    setLoading(true);
    const [error, response] = await createOrganization(trimmed);
    setLoading(false);

    if (error) {
      setFormError(translateApiError(error.message));
      return;
    }

    handleClose();
    router.push(`/orgs/${response!.id}`);
  };

  return (
    <>
      <Button leftSection={<IconPlus size={18} />} onClick={open}>
        Создать организацию
      </Button>
      <Modal opened={opened} onClose={handleClose} title="Новая организация" size="xs" centered>
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
            label="Название"
            placeholder="Моя организация"
            value={name}
            error={nameError}
            onChange={(e) => {
              setName(e.currentTarget.value);
              setNameError(null);
              setFormError(null);
            }}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
            autoFocus
          />
          <Button loading={loading} onClick={handleCreate} disabled={!name.trim()}>
            Создать
          </Button>
        </Stack>
      </Modal>
    </>
  );
}
