// Middleware disabled - using Next.js rewrites for Kratos proxy instead
// The @ory/nextjs middleware adds ory-base-url-rewrite headers that cause
// redirect loops when ory-base-url-rewrite-token is not configured on Kratos

import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

export function middleware(request: NextRequest) {
  return NextResponse.next();
}

export const config = {
  matcher: [],
}