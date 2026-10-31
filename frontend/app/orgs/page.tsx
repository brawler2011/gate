import {Container, Group, SimpleGrid, Stack, Text, Title, Center} from "@mantine/core";
import {useLoaderData} from "react-router-dom";

import {CreateOrgButton, OrgCard} from "@/components/orgs";
import {DefaultLayout} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";

import type {ReactNode} from "react";
import type {LoaderFunction} from "react-router-dom";

export const loader = (async () => {
  const [{data: me}, {data, error: orgsErr, response: orgsResponse}] = await Promise.all([
    api.GET("/users/me"),
    api.GET("/organizations", {params: {query: {page: 1, pageSize: 50}}}),
  ]);
  const user = me?.user ?? null;
  const error = orgsErr
    ? {
      status: orgsResponse.status,
      message: orgsErr.message || orgsResponse.statusText,
    }
    : null;
  return {user, error, data};
}) satisfies LoaderFunction;

const OrgsPage = (): ReactNode => {
  const loaded = useLoaderData<typeof loader>();
  const {user, error, data} = loaded;
  if (error) {
    return (
      <DefaultLayout headerUser={user}>
        <Container size="lg" py="lg">
          <ErrorDisplay error={error} />
        </Container>
      </DefaultLayout>
    );
  }
  const orgs = data!.organizations;
  return (
    <DefaultLayout headerUser={user}>
      <Container size="lg" py="lg">
        <Stack gap="md">
          <Group justify="space-between" align="flex-end">
            <Title order={2}>Организации</Title>
            {user && <CreateOrgButton />}
          </Group>
          {orgs.length === 0 ? (
            <Center py="xl">
              <Text c="dimmed">У вас пока нет организаций</Text>
            </Center>
          ) : (
            <SimpleGrid cols={{base: 1, xs: 2, sm: 2, md: 3, lg: 4}} spacing="md">
              {orgs.map((org) => (
                <OrgCard key={org.id} org={org} />
              ))}
            </SimpleGrid>
          )}
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export default OrgsPage;
