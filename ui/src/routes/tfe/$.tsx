import { createFileRoute } from '@tanstack/react-router'

async function handler({ request }) {
  const url = new URL(request.url);
  const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}${url.pathname}${url.search}`, {
    method: request.method,
    headers: request.headers,
    body: request.method !== 'GET' ? await request.blob() : undefined
  });
  return response;
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