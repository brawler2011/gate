import fs from 'fs';
import path from 'path';
import matter from 'gray-matter';

export interface BlogPostMetadata {
  id: string;
  title: string;
  author: string;
  avatarUrl?: string;
  date: string;
  description: string;
  previewImageUrl?: string;
}

export interface BlogPost extends BlogPostMetadata {
  content: string;
}

const contentDirectory = path.join(process.cwd(), 'content', 'blog');

/**
 * Get all blog posts metadata
 */
export function getAllBlogPosts(): BlogPostMetadata[] {
  const files = fs.readdirSync(contentDirectory);
  const mdxFiles = files.filter(file => file.endsWith('.mdx'));

  const posts = mdxFiles.map((file) => {
    const filePath = path.join(contentDirectory, file);
    const fileContent = fs.readFileSync(filePath, 'utf8');
    const { data } = matter(fileContent);

    return data as BlogPostMetadata;
  });

  // Sort by date (newest first)
  return posts.sort((a, b) => new Date(b.date).getTime() - new Date(a.date).getTime());
}

/**
 * Get a single blog post by ID
 */
export function getBlogPost(id: string): BlogPost | null {
  try {
    const filePath = path.join(contentDirectory, `${id}.mdx`);
    const fileContent = fs.readFileSync(filePath, 'utf8');
    const { data, content } = matter(fileContent);

    return {
      ...(data as BlogPostMetadata),
      content,
    };
  } catch (error) {
    console.error(`Failed to load blog post ${id}:`, error);
    return null;
  }
}

/**
 * Get all blog post IDs for static generation
 */
export function getBlogPostIds(): string[] {
  const files = fs.readdirSync(contentDirectory);
  return files
    .filter(file => file.endsWith('.mdx'))
    .map(file => file.replace('.mdx', ''));
}

