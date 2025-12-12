import { DefaultLayout } from "@/components/Layout";
import { isAuthenticated } from "@/lib/auth";
import { listPosts } from "@/lib/actions";
import {
  Container,
  Group,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { IconNews } from "@tabler/icons-react";
import { BlogPost } from "@/components/BlogPost/BlogPost";

export const metadata = {
  title: "Главная",
};

export default async function Page() {
  const authenticated = await isAuthenticated();
  
  // Fetch blog posts from API
  const [error, postsData] = await listPosts(1, 20);
  const blogPosts = postsData?.posts || [];

  const renderBlogSection = () => (
    <Stack gap="md">
      <Group gap="xs">
        <IconNews size={28} color="var(--mantine-color-orange-6)" />
        <Title order={2}>Блог</Title>
      </Group>
      {error ? (
        <Text c="dimmed">Не удалось загрузить посты</Text>
      ) : blogPosts.length === 0 ? (
        <Text c="dimmed">Пока нет постов</Text>
      ) : (
        <Stack gap="md">
          {blogPosts.map((post) => (
            <BlogPost
              key={post.id}
              id={post.id || ""}
              title={post.title || "Без названия"}
              author={post.author_username || "Аноним"}
              date={post.created_at}
              description={post.description || ""}
              useApiImage={!!post.preview_image_id}
            />
          ))}
        </Stack>
      )}
    </Stack>
  );

  return (
    <DefaultLayout>
      <Container size="lg" py="xl">
        {authenticated ? (
          <Stack gap="md">
            {renderBlogSection()}
          </Stack>
        ) : (
          renderBlogSection()
        )}
      </Container>
    </DefaultLayout>
  );
}
