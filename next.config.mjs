import createMDX from '@next/mdx'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    eslint: {
        ignoreDuringBuilds: true,
    },
    typescript: {
        ignoreBuildErrors: true,
    },
    pageExtensions: ['js', 'jsx', 'md', 'mdx', 'ts', 'tsx'],
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
        const oryUrl = process.env.ORY_SDK_URL;
        console.log('🔧 ORY_SDK_URL =', oryUrl);
        console.log('🔧 NEXT_PUBLIC_ORY_SDK_URL =', process.env.NEXT_PUBLIC_ORY_SDK_URL);
        console.log('🔧 TESTER_URL =', process.env.TESTER_URL);
        console.log('🔧 GATEWAY_URL =', process.env.GATEWAY_URL);
        console.log('🔧 BACKEND_API_URL =', process.env.BACKEND_API_URL);
        console.log('🔧 NEXT_PUBLIC_WS_core_URL =', process.env.NEXT_PUBLIC_WS_core_URL);

        if (!oryUrl) {
            console.warn('⚠️  ORY_SDK_URL is not set! Auth will not work.');
            return [];
        }
        return [
            {
                source: '/api/.ory/:path*',
                destination: `${oryUrl}/:path*`,
            },
        ];
    },
}

const withMDX = createMDX({
    // Add markdown plugins here, as desired
})

export default withMDX(nextConfig)
