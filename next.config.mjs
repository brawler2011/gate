import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    typescript: {
        ignoreBuildErrors: true,
    },
    eslint: {
        ignoreDuringBuilds: true,
    },
    pageExtensions: ['js', 'jsx', 'ts', 'tsx'],
    experimental: {
        serverActions: {
            bodySizeLimit: '20mb',
        },
    },
    
    webpack: (config) => {
        config.resolve.alias = {
            ...config.resolve.alias,
            '@contracts': path.resolve(__dirname, '../contracts'),
        }
        return config
    },
    async rewrites() {
        const backendUrl = process.env.BACKEND_API_URL;
        console.log('🔧 BACKEND_API_URL =', backendUrl);
        console.log('🔧 WEBSOCKET_URL =', process.env.WEBSOCKET_URL);

        if (!backendUrl) {
            console.warn('⚠️  BACKEND_API_URL is not set! Auth might not work if it depends on rewrites.');
            return [];
        }
        return [
            {
                source: '/api/.ory/:path*',
                destination: `${backendUrl}/api/.ory/:path*`,
            },
        ];
    },
}

export default nextConfig
