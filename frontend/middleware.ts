import {NextResponse} from "next/server";

import type {NextRequest} from "next/server";

export const middleware = (request: NextRequest): NextResponse => {
  const {pathname} = request.nextUrl;

  // Block direct access to internal /user routes from browser
  if (pathname === "/user" || pathname.startsWith("/user/")) {
    return new NextResponse(null, {status: 404});
  }

  // Handle /@username or /%40username
  if (pathname.startsWith("/@") || pathname.startsWith("/%40")) {
    let cleanUsername = "";
    try {
      cleanUsername = decodeURIComponent(pathname).slice(2);
    } catch {
      return new NextResponse(null, {status: 404});
    }

    // Disallow empty username or nested paths for now
    if (!cleanUsername || cleanUsername.includes("/")) {
      return new NextResponse(null, {status: 404});
    }

    const url = request.nextUrl.clone();
    url.pathname = `/user/${encodeURIComponent(cleanUsername)}`;
    return NextResponse.rewrite(url);
  }

  return NextResponse.next();
};

export const config = {
  matcher: [
    /*
     * Match all request paths except for the ones starting with:
     * - api (API routes)
     * - _next/static (static files)
     * - _next/image (image optimization files)
     * - favicon.ico (favicon file)
     * - public assets (svg, png, jpg, etc.)
     */
    "/((?!api|_next/static|_next/image|favicon.ico|.*\\.(?:svg|png|jpg|jpeg|gif|webp)$).*)",
  ],
};
