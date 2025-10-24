import { createFileRoute } from '@tanstack/react-router'

async function handler({ request }) {
  const url = new URL(request.url);
  const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}${url.pathname}${url.search}`, {
    method: request.method,
    headers: request.headers,
    // body: request.method !== 'GET' ? await request.blob() : undefined
  });

// important, remove all encoding headers since the fetch already decompresses the gzip
// the removal of headeres avoids gzip errors in the client
const headers = new Headers(response.headers);
headers.delete('Content-Encoding');
headers.delete('content-length');
headers.delete('transfer-encoding');
headers.delete('connection');

  return new Response(response.body, { headers });
}

export const Route = createFileRoute('/tfe/$')({
  server: {
    handlers: {
      GET: handler,
      POST: handler,
      PUT: handler,
      DELETE: handler,
      PATCH: handler,
      HEAD: handler,
      OPTIONS: handler,
      LOCK: handler,
      UNLOCK: handler
    }
  }
})