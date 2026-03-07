import { DefaultLayout } from '@/components/shared';
import { OrgCard } from '@/components/orgs/OrgCard';
import { CreateOrgButton } from '@/components/orgs/CreateOrgButton';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { listOrganizations } from '@/lib/actions';
import { getCurrentUser } from '@/lib/auth';
import { Container, Group, SimpleGrid, Stack, Text, Title, Center } from '@mantine/core';
import type { Metadata } from 'next';

export const metadata: Metadata = { title: 'Организации' };

export default async function OrgsPage() {
  const user = await getCurrentUser();
  const [error, data] = await listOrganizations(1, 50);
  if (error) return <DefaultLayout><Container size="lg" py="lg"><ErrorDisplay error={error} /></Container></DefaultLayout>;

  const orgs = data!.organizations;

  return (
    <DefaultLayout>
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
            <SimpleGrid cols={{ base: 1, xs: 2, sm: 2, md: 3, lg: 4 }} spacing="md">
              {orgs.map((org) => <OrgCard key={org.id} org={org} />)}
            </SimpleGrid>
          )}
        </Stack>
      </Container>
    </DefaultLayout>
  );
}
