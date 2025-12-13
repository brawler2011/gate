import { NextRequest, NextResponse } from 'next/server';
import { cookies } from 'next/headers';
import { gateway } from '@contracts/gateway/v1';

const oryKratosCookieName = 'ory_kratos_session';

/**
 * API route для получения изображений постов через Gateway клиент
 * Использует сгенерированный клиент вместо прямого fetch
 */
export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  
  // Получаем куку сессии Kratos
  const requestCookies = await cookies();
  const kratosCookie = requestCookies.get(oryKratosCookieName);
  
  const headers: Record<string, string> = {};
  if (kratosCookie) {
    headers['Cookie'] = `${oryKratosCookieName}=${kratosCookie.value}`;
  }

  const backendUrl = process.env.BACKEND_API_URL;
  if (!backendUrl) {
    console.error('BACKEND_API_URL is not set');
    return new NextResponse('Backend URL not configured', { status: 500 });
  }

  try {
    // Для бинарных данных (изображений) используем прямой fetch вместо OpenAPI клиента,
    // чтобы избежать проблем с конвертацией raw binary в JavaScript строку
    const imageUrl = `${backendUrl}/blogs/posts/${id}/image`;
    
    const fetchResponse = await fetch(imageUrl, {
      method: 'GET',
      headers: headers,
      credentials: 'include',
    });

    if (!fetchResponse.ok) {
      throw new Error(`HTTP error! status: ${fetchResponse.status}`);
    }

    const imageBuffer = await fetchResponse.arrayBuffer();
    
    return new NextResponse(imageBuffer, {
      status: 200,
      headers: {
        'Content-Type': 'image/png',
        'Cache-Control': 'public, max-age=31536000, immutable',
      },
    });
  } catch (error) {
    console.error(`Error fetching image for post ${id}:`, error);
    return new NextResponse('Image not found', { status: 404 });
  }
}



