import { NextRequest, NextResponse } from "next/server";
import { listPosts } from "@/lib/actions";

export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const page = Number(searchParams.get("page")) || 1;

  const [err, res] = await listPosts(page, 10);

  if (err) {
    return NextResponse.json(
      { message: err.message || "Не удалось загрузить посты" },
      { status: err.status || 500 }
    );
  }

  return NextResponse.json(res);
}
