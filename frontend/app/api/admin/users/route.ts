import { NextRequest, NextResponse } from "next/server";
import { listUsers } from "@/lib/actions";

export async function GET(request: NextRequest) {
  const searchParams = request.nextUrl.searchParams;
  const page = Number(searchParams.get("page")) || 1;
  const search = searchParams.get("search") || undefined;
  const role = searchParams.get("role") || undefined;

  const [err, res] = await listUsers(page, 10, search, role);

  if (err) {
    return NextResponse.json(
      { message: err.message || "Не удалось загрузить пользователей" },
      { status: err.status || 500 }
    );
  }

  return NextResponse.json(res);
}
