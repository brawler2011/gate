// FIXME: burn this

import { cookies } from 'next/headers';
import { NextRequest, NextResponse } from 'next/server';

const sessionCookieName = 'session_id';

export async function GET(
  _: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const { id } = await params;
  
  const requestCookies = await cookies();
  const sessionCookie = requestCookies.get(sessionCookieName);
  
  const headers: Record<string, string> = {};
  if (sessionCookie) {
    headers['Cookie'] = `${sessionCookieName}=${sessionCookie.value}`;
  }

  const backendUrl = process.env.BACKEND_API_URL;
  if (!backendUrl) {
    console.error('BACKEND_API_URL is not set');
    return new NextResponse('Backend URL not configured', { status: 500 });
  }

  try {
    const imageUrl = `${backendUrl}/posts/${id}/image`;
    
    console.log('imageUrl', imageUrl);

    const fetchResponse = await fetch(imageUrl, {
      method: 'GET',
      headers: headers,
      credentials: 'include',
    });

    if (!fetchResponse.ok) {
      throw new Error(`HTTP error! status: ${fetchResponse.status}`);
    }

    const imageBuffer = await fetchResponse.arrayBuffer();
    const contentType = fetchResponse.headers.get('Content-Type') || 'image/png';
    
    return new NextResponse(imageBuffer, {
      status: 200,
      headers: {
        'Content-Type': contentType,
        'Cache-Control': 'public, max-age=31536000, immutable',
      },
    });
  } catch (error) {
    console.error(`Error fetching image for post ${id}:`, error);
    return new NextResponse('Image not found', { status: 404 });
  }
}
