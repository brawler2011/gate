/** @type {import('next').NextConfig} */
const nextConfig = {
    output: 'standalone',
    pageExtensions: ['js', 'jsx', 'ts', 'tsx'],
    experimental: {
        serverActions: {
            bodySizeLimit: '20mb',
        },
    },
}

export default nextConfig
