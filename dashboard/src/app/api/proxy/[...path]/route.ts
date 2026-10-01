import { NextRequest, NextResponse } from "next/server";

const GATEWAY_BASE = process.env.GATEWAY_URL || "http://127.0.0.1:8050";

export async function GET(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  const targetUrl = `${GATEWAY_BASE}/${path.join("/")}${req.nextUrl.search}`;

  try {
    const res = await fetch(targetUrl, {
      cache: "no-store",
    });

    const contentType = res.headers.get("content-type") || "";
    if (contentType.includes("text/csv") || res.headers.get("content-disposition")) {
      const headers = new Headers();
      headers.set("Content-Type", contentType || "text/csv; charset=utf-8");
      const disp = res.headers.get("content-disposition");
      if (disp) {
        headers.set("Content-Disposition", disp);
      }
      return new Response(res.body, {
        status: res.status,
        headers,
      });
    }

    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch (err: any) {
    return NextResponse.json(
      { error: err.message || "Failed to reach gateway" },
      { status: 502 }
    );
  }
}

export async function POST(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  const targetUrl = `${GATEWAY_BASE}/${path.join("/")}${req.nextUrl.search}`;

  try {
    const body = await req.json().catch(() => ({}));
    const res = await fetch(targetUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    });

    const isSSE = res.headers.get("content-type")?.includes("text/event-stream");
    if (isSSE && res.body) {
      return new Response(res.body, {
        headers: {
          "Content-Type": "text/event-stream",
          "Cache-Control": "no-cache",
          Connection: "keep-alive",
        },
      });
    }

    const data = await res.json();
    return NextResponse.json(data, { status: res.status });
  } catch (err: any) {
    return NextResponse.json(
      { error: err.message || "Failed to post to gateway" },
      { status: 502 }
    );
  }
}

export async function DELETE(
  req: NextRequest,
  { params }: { params: Promise<{ path: string[] }> }
) {
  const { path } = await params;
  const targetUrl = `${GATEWAY_BASE}/${path.join("/")}${req.nextUrl.search}`;

  try {
    const res = await fetch(targetUrl, {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json",
      },
    });
    const data = await res.json().catch(() => ({ ok: res.ok }));
    return NextResponse.json(data, { status: res.status });
  } catch (err: any) {
    return NextResponse.json(
      { error: err.message || "Failed to delete resource" },
      { status: 502 }
    );
  }
}
