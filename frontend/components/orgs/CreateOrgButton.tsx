"use client";

import {Alert, Button, Modal, TextInput, Stack} from '@mantine/core';
import {useDisclosure} from '@mantine/hooks';
import {IconAlertCircle, IconPlus} from '@tabler/icons-react';
import {useRouter} from 'next/navigation';
import {useState} from 'react';

import {api} from '@/lib/api';

import type {ReactNode} from "react";

const translateApiError = (message: string): string => {
  if (message.includes('at least one latin letter or digit')) {
    return 'Название должно содержать хотя бы одну латинскую букву или цифру';
  }
  if (message.includes('between 3 and 64 characters')) {
    return 'Название должно быть от 3 до 64 символов';
  }
  if (message.includes('between 3 and 32 characters')) {
    return 'Логин должен быть от 3 до 32 символов';
  }
  if (message.includes('cannot start with \'@\'')) {
    return 'Логин организации не может начинаться с @';
  }
  if (message.includes('is reserved')) {
    return 'Этот логин зарезервирован системой';
  }
  if (message.includes('failed to create organization')) {
    return 'Не удалось создать организацию. Попробуйте позже';
  }
  return message;
};

const generateSlug = (val: string): string => {
  return val
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .replace(/-+/g, '-');
};

export const CreateOrgButton = (): ReactNode => {
  const [opened, {open, close}] = useDisclosure(false);
  const [name, setName] = useState('');
  const [login, setLogin] = useState('');
  const [loginEdited, setLoginEdited] = useState(false);
  const [loading, setLoading] = useState(false);
  const [nameError, setNameError] = useState<string | null>(null);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const router = useRouter();

  const handleClose = () => {
    close();
    setName('');
    setLogin('');
    setLoginEdited(false);
    setNameError(null);
    setLoginError(null);
    setFormError(null);
  };

  const handleNameChange = (val: string) => {
    setName(val);
    setNameError(null);
    setFormError(null);
    if (!loginEdited) {
      setLogin(generateSlug(val));
    }
  };

  const handleLoginChange = (val: string) => {
    setLogin(val);
    setLoginEdited(true);
    setLoginError(null);
    setFormError(null);
  };

  const handleCreate = async () => {
    const trimmedName = name.trim();
    const trimmedLogin = login.trim();
    if (!trimmedName) {
      return;
    }

    if (trimmedName.length < 3) {
      setNameError('Название должно содержать не менее 3 символов');
      return;
    }
    if (trimmedName.length > 64) {
      setNameError('Название не должно превышать 64 символа');
      return;
    }

    if (trimmedLogin && trimmedLogin.length < 3) {
      setLoginError('Логин должен содержать не менее 3 символов');
      return;
    }
    if (trimmedLogin && trimmedLogin.length > 32) {
      setLoginError('Логин не должен превышать 32 символа');
      return;
    }

    setLoading(true);
    const [error, response] = await api.createOrganization({
      name: trimmedName,
      login: trimmedLogin || undefined,
    });
    setLoading(false);

    if (error) {
      setFormError(translateApiError(error.message));
      return;
    }

    handleClose();
    router.push(`/${response!.login}`);
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
            onChange={(e) => handleNameChange(e.currentTarget.value)}
            autoFocus
          />
          <TextInput
            label="Логин (URL)"
            placeholder="my-org"
            value={login}
            error={loginError}
            onChange={(e) => handleLoginChange(e.currentTarget.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          />
          <Button loading={loading} onClick={handleCreate} disabled={!name.trim()}>
            Создать
          </Button>
        </Stack>
      </Modal>
    </>
  );
};
