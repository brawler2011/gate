import { NextRequest, NextResponse } from "next/server";
import { listAdminContests } from "@/lib/actions";

export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const page = Number(searchParams.get("page")) || 1;
  const search = searchParams.get("search") || undefined;

  const [err, res] = await listAdminContests(page, 10, search);

  if (err) {
    return NextResponse.json(
      { message: err.message || "Не удалось загрузить контесты" },
      { status: err.status || 500 }
    );
  }

  return NextResponse.json(res);
}
