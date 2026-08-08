import type { MetadataRoute } from 'next';

// FIXME: hardcoded domain
export default function robots(): MetadataRoute.Robots {
  return {
    rules: [
      {
        userAgent: '*',
        allow: '/',
        disallow: ['/auth/', '/admin/']
      }
    ],
    sitemap: 'https://gate149.ru/sitemap.xml'
  };
}
