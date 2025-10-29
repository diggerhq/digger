import { verifyTokenFn } from '@/api/tokens_serverFunctions';
import { createFileRoute } from '@tanstack/react-router'

async function handler({ request }) {
  const url = new URL(request.url);
  
  try {
    const token = request.headers.get('authorization')?.split(' ')[1]
    const tokenValidation = await verifyTokenFn({data: { token: token}})
    if (!tokenValidation.valid) {
      return new Response('Unauthorized', { status: 401 })
    }
  } catch (error) {
    console.error('Error verifying token', error)
    return new Response('Unauthorized', { status: 401 })
  }


  // important: we need to set these to allow the statesman backend to return the correct URL to opentofu or terraform clients
  const outgoingHeaders = new Headers(request.headers);
  const originalHost = outgoingHeaders.get('host') ?? '';
  console.log('originalHost', originalHost);
  if (originalHost) outgoingHeaders.set('x-forwarded-host', originalHost);
  outgoingHeaders.set('x-forwarded-proto', url.protocol.replace(':', ''));
  if (url.port) outgoingHeaders.set('x-forwarded-port', url.port);
  // Let fetch manage these, and drop hop-by-hop headers
  ['host','content-length','connection','keep-alive','proxy-connection','transfer-encoding','upgrade','te','trailer','accept-encoding']
  .forEach(h => outgoingHeaders.delete(h));


  const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}${url.pathname}${url.search}`, {
    method: request.method,
    headers: request.headers,
    body: request.method !== 'GET' ? await request.blob() : undefined
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