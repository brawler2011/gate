import { DefaultLayout } from "@/components/Layout";
import { isAuthenticated } from "@/lib/auth";
import {
  Container,
  Group,
  Stack,
  Title,
} from "@mantine/core";
import { IconNews } from "@tabler/icons-react";
import { BlogPost } from "@/components/BlogPost/BlogPost";
import { getAllBlogPosts } from "@/lib/blog";

export const metadata = {
  title: "Главная",
};

export default async function Page() {
  const authenticated = await isAuthenticated();
  const blogPosts = getAllBlogPosts();

  return (
    <DefaultLayout>
      <Container size="lg" py="xl">
        {authenticated ? (
          <Stack gap="md">
            {/* Blog Section - Full width below */}
            <Stack gap="md">
              <Group gap="xs">
                <IconNews size={28} color="var(--mantine-color-orange-6)" />
                <Title order={2}>Блог</Title>
              </Group>
              <Stack gap="md">
                {blogPosts.map((post) => (
                  <BlogPost
                    key={post.id}
                    id={post.id}
                    title={post.title}
                    author={post.author}
                    avatarUrl={post.avatarUrl}
                    date={post.date}
                    description={post.description}
                    previewImageUrl={post.previewImageUrl}
                  />
                ))}
              </Stack>
            </Stack>
          </Stack>
        ) : (
          /* For non-authenticated users - only show Blog */
          <Stack gap="md">
            <Group gap="xs">
              <IconNews size={28} color="var(--mantine-color-orange-6)" />
              <Title order={2}>Блог</Title>
            </Group>
            <Stack gap="md">
              {blogPosts.map((post) => (
                <BlogPost
                  key={post.id}
                  id={post.id}
                  title={post.title}
                  author={post.author}
                  avatarUrl={post.avatarUrl}
                  date={post.date}
                  description={post.description}
                  previewImageUrl={post.previewImageUrl}
                />
              ))}
            </Stack>
          </Stack>
        )}
      </Container>
    </DefaultLayout>
  );
}
