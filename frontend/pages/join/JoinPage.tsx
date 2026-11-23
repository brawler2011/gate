import {Button, Card, Center, Container, Group, Loader, Stack, Text, Title} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useMutation, useQuery, useQueryClient} from "@tanstack/react-query";
import {Link, useLocation, useNavigate, useParams} from "react-router-dom";

import {DefaultLayout} from "@/components/shared";
import {useSession} from "@/contexts/SessionContext";
import {api, unwrap} from "@/lib/api";
import {problemMessage} from "@/lib/problemText";

import type {JoinLinkTargetModel} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const targetPath = (t: JoinLinkTargetModel): string =>
  t.kind === "team" ? `/${t.organization_login}/teams/${t.id}` : `/${t.organization_login}/${t.contest_login}`;

const JoinPage = (): ReactNode => {
  const {token = ""} = useParams();
  const {user, isAuthenticated, isLoading} = useSession();
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const target = useQuery({
    queryKey: ["join-link", token],
    queryFn: async () => {
      const {data, error} = await api.GET("/join-links/{token}", {params: {path: {token}}});
      if (error || !data) {
        throw new Error(problemMessage(error, "Ссылка недействительна"));
      }
      return data;
    },
    enabled: isAuthenticated,
    retry: false,
  });
  const follow = useMutation({
    mutationFn: () => unwrap(api.POST("/join-links/{token}", {params: {path: {token}}})),
    onSuccess: async (joined) => {
      await queryClient.invalidateQueries();
      navigate(targetPath(joined));
    },
    onError: (error: Error) => notifications.show({title: "Не удалось присоединиться", message: error.message, color: "red"}),
  });

  const returnTo = encodeURIComponent(location.pathname);
  let content: ReactNode;
  if (isLoading || target.isLoading) {
    content = <Center py="xl"><Loader /></Center>;
  } else if (!isAuthenticated) {
    content = (
      <Stack>
        <Text>Войдите или зарегистрируйтесь, чтобы присоединиться по этой ссылке.</Text>
        <Group>
          <Button component={Link} to={`/auth/login?return_to=${returnTo}`}>Войти</Button>
          <Button variant="light" component={Link} to={`/auth/registration?return_to=${returnTo}`}>Зарегистрироваться</Button>
        </Group>
      </Stack>
    );
  } else if (target.error || !target.data) {
    content = <Text c="red">Ссылка недействительна: её выключили или заменили. Попросите новую у учителя или организатора.</Text>;
  } else {
    const t = target.data;
    content = (
      <Stack>
        {t.kind === "team" ? (
          <Text>
            Вы вступите в команду <b>{t.team_name}</b> организации <b>{t.organization_name}</b>.
          </Text>
        ) : (
          <Text>
            Вы станете участником контеста <b>{t.contest_title}</b> организации <b>{t.organization_name}</b>.
          </Text>
        )}
        <Group>
          <Button onClick={() => follow.mutate()} loading={follow.isPending}>Присоединиться</Button>
        </Group>
      </Stack>
    );
  }

  return (
    <DefaultLayout headerUser={user}>
      <title>Приглашение</title>
      <Container size="sm" py="xl">
        <Card withBorder p="lg">
          <Stack>
            <Title order={3}>Приглашение</Title>
            {content}
          </Stack>
        </Card>
      </Container>
    </DefaultLayout>
  );
};

export default JoinPage;
