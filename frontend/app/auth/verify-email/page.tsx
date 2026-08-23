"use client";

import {
  Alert,
  Box,
  Button,
  Center,
  Group,
  Loader,
  Paper,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {IconAlertCircle, IconCheck} from "@tabler/icons-react";
import Image from "next/image";
import Link from "next/link";
import {useSearchParams} from "next/navigation";
import {Suspense, useEffect, useState, type ReactNode} from "react";

import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

const VerifyEmailPage = (): ReactNode => {
  return (
    <Suspense
      fallback={
        <Center h="100vh">
          <Loader size="lg" />
        </Center>
      }
    >
      <VerifyEmailContent />
    </Suspense>
  );
};

const VerifyEmailContent = () => {
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";
  const {setUser} = useSession();

  const [loading, setLoading] = useState(true);
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) {
      setError("Токен подтверждения отсутствует в ссылке.");
      setLoading(false);
      return;
    }

    let isMounted = true;

    const verify = async () => {
      try {
        const [err, data] = await api.verifyEmail({
          requestBody: {token},
        });

        if (!isMounted) {
          return;
        }

        if (!err && data) {
          if (data.user) {
            setUser(data.user);
          }
          setSuccess(true);
        } else {
          setError(err?.message || "Недействительная или устаревшая ссылка подтверждения.");
        }
      } catch {
        if (isMounted) {
          setError("Не удалось подключиться к серверу");
        }
      } finally {
        if (isMounted) {
          setLoading(false);
        }
      }
    };

    verify();

    return () => {
      isMounted = false;
    };
  }, [token, setUser]);

  const renderContent = (): ReactNode => {
    if (loading) {
      return (
        <Stack align="center" gap="md" py="xl">
          <Loader size="lg" />
          <Text fz={15} c="dimmed">
            Подтверждаем ваш адрес электронной почты...
          </Text>
        </Stack>
      );
    }

    if (success) {
      return (
        <Stack align="center" gap="md" ta="center">
          <ThemeIcon size={64} radius="xl" color="green" variant="light">
            <IconCheck size={36} />
          </ThemeIcon>

          <Title order={2} fz={22}>
            Почта успешно подтверждена!
          </Title>

          <Text c="dimmed" fz={15}>
            Ваша учетная запись активирована. Добро пожаловать на платформу Gate.
          </Text>

          <Button
            component={Link}
            href="/"
            variant="filled"
            size="sm"
            mt="md"
          >
            Перейти на главную
          </Button>
        </Stack>
      );
    }

    return (
      <Stack align="center" gap="md" ta="center">
        <ThemeIcon size={64} radius="xl" color="red" variant="light">
          <IconAlertCircle size={36} />
        </ThemeIcon>

        <Title order={2} fz={22}>
          Не удалось подтвердить почту
        </Title>

        {error && (
          <Alert
            icon={<IconAlertCircle size={18} />}
            color="red"
            title="Ошибка верификации"
            radius="md"
            w="100%"
            ta="left"
          >
            {error}
          </Alert>
        )}

        <Group justify="center" mt="md" gap="sm">
          <Button
            component={Link}
            href="/auth/login"
            variant="outline"
            size="sm"
          >
            Перейти ко входу
          </Button>
          <Button
            component={Link}
            href="/auth/registration"
            variant="filled"
            size="sm"
          >
            Зарегистрироваться
          </Button>
        </Group>
      </Stack>
    );
  };

  return (
    <Box
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "1rem",
      }}
    >
      <Stack
        align="center"
        gap={32}
        style={{width: "min(550px, calc(100vw - 2rem))"}}
      >
        <Link href="/" style={{textDecoration: "none", color: "inherit"}}>
          <Group justify="center" gap="md">
            <Image
              src="/gate_logo.svg"
              alt="Gate"
              width={56}
              height={56}
              priority
            />
            <Title order={1} fz={36}>
              Gate
            </Title>
          </Group>
        </Link>

        <Paper
          radius="md"
          p={32}
          withBorder
          shadow="sm"
          style={{width: "100%"}}
        >
          {renderContent()}
        </Paper>
      </Stack>
    </Box>
  );
};

export default VerifyEmailPage;
