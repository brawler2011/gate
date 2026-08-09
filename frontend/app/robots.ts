import {env} from '@/lib/env';

import type {MetadataRoute} from 'next';

const robots = (): MetadataRoute.Robots => {
  const baseUrl = env.getAppUrl();

  return {
    rules: [
      {
        userAgent: '*',
        allow: '/',
        disallow: ['/auth/', '/admin/'],
      },
    ],
    sitemap: `${baseUrl}/sitemap.xml`,
  };
};

export default robots;
