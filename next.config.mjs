import createMDX from '@next/mdx'

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
    async rewrites() {
        const oryUrl = process.env.ORY_SDK_URL;
        console.log('🔧 ORY_SDK_URL =', oryUrl);
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
