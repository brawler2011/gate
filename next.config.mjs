import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    cacheComponents: true,
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
    async rewrites() {
        return [
            {
                source: '/api/.ory/:path*',
                destination: `${process.env.BACKEND_API_URL}/api/.ory/:path*`,
            },
        ];
    },
}

export default nextConfig
