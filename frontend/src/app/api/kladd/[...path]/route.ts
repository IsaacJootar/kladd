import { NextRequest, NextResponse } from "next/server";

const apiBaseURL = process.env.KLADD_API_BASE_URL ?? "http://localhost:8080";

type RouteContext = {
  params: Promise<{
    path: string[];
  }>;
};

export async function GET(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context);
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxyRequest(request, context);
}

async function proxyRequest(request: NextRequest, context: RouteContext) {
  const { path } = await context.params;
  const targetURL = new URL(`/api/${path.join("/")}`, apiBaseURL);

  const headers = new Headers();
  const contentType = request.headers.get("content-type");
  const authorization = request.headers.get("authorization");
  if (contentType) {
    headers.set("content-type", contentType);
  }
  if (authorization) {
    headers.set("authorization", authorization);
  }

  const hasBody = request.method !== "GET" && request.method !== "HEAD";
  const response = await fetch(targetURL, {
    method: request.method,
    headers,
    body: hasBody ? await request.text() : undefined,
    cache: "no-store",
  });

  const responseContentType =
    response.headers.get("content-type") ?? "application/json";

  return new NextResponse(await response.text(), {
    status: response.status,
    headers: {
      "content-type": responseContentType,
    },
  });
}
