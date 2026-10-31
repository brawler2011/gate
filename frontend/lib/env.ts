export const env = {
  isProduction: (): boolean => import.meta.env.PROD,
  isDevelopment: (): boolean => import.meta.env.DEV,
  getBackendApiUrl: (): string => '/api',
  getWebSocketUrl: (): string => {
    const url = new URL('/ws', window.location.origin);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
    return url.toString().replace(/\/$/, '');
  },
  getAppUrl: (): string => window.location.origin,
} as const;
