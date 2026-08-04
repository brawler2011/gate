/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    pageExtensions: ['js', 'jsx', 'ts', 'tsx'],
    experimental: {
        serverActions: {
            bodySizeLimit: '50mb',
        },
    },
    async rewrites() {
        return [
            {
                source: '/api/:path*',
                destination: `${process.env.BACKEND_API_URL || 'http://localhost:8080'}/:path*`,
            },
        ];
    },
}

export default nextConfig
