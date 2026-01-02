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
    turbopack: {
        resolveAlias: {
            '@contracts': './contracts',
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
