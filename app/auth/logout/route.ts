import { NextResponse } from "next/server";
import { cookies } from "next/headers";

export async function GET() {
  const response = NextResponse.redirect(
    new URL("/auth/login", process.env.NEXT_PUBLIC_APP_URL || "http://localhost:3000")
  );

  // Clear all Ory-related cookies
  const cookieStore = await cookies();
  const allCookies = cookieStore.getAll();
  
  for (const cookie of allCookies) {
    if (cookie.name.startsWith("ory_") || cookie.name.startsWith("csrf_token")) {
      response.cookies.delete(cookie.name);
    }
  }

  return response;
}
