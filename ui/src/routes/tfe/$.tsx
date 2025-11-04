import { createFileRoute } from '@tanstack/react-router'
import { tokenCache } from '@/lib/token-cache.server'

async function handler({ request }) {
  const url = new URL(request.url);
  
  // OAuth/discovery paths that don't require token auth (login flow)
  const isOAuthPath = 
    url.pathname.startsWith('/tfe/app/oauth2/') ||
    url.pathname.startsWith('/tfe/oauth2/') ||
    url.pathname === '/.well-known/terraform.json' ||
    url.pathname === '/tfe/api/v2/motd';
  
  // Upload paths that use signed URLs (no Bearer token)
  const isUploadPath =
    /^\/tfe\/api\/v2\/state-versions\/[^\/]+\/upload$/.test(url.pathname) ||
    /^\/tfe\/api\/v2\/state-versions\/[^\/]+\/json-upload$/.test(url.pathname);

  // OAuth and upload paths: forward directly to public statesman endpoints
  if (isOAuthPath || isUploadPath) {
    const outgoingHeaders = new Headers(request.headers);
    const originalHost = outgoingHeaders.get('host') ?? '';
    if (originalHost) outgoingHeaders.set('x-forwarded-host', originalHost);
    outgoingHeaders.set('x-forwarded-proto', url.protocol.replace(':', ''));
    if (url.port) outgoingHeaders.set('x-forwarded-port', url.port);
    
    // Drop hop-by-hop headers (but KEEP accept-encoding for compression)
    ['host','content-length','connection','keep-alive','proxy-connection','transfer-encoding','upgrade','te','trailer']
      .forEach(h => outgoingHeaders.delete(h));

    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}${url.pathname}${url.search}`, {
      method: request.method,
      headers: outgoingHeaders,
      body: request.method !== 'GET' && request.method !== 'HEAD' ? await request.blob() : undefined
    });

    const headers = new Headers(response.headers);
    headers.delete('Content-Encoding');
    headers.delete('content-length');
    headers.delete('transfer-encoding');
    headers.delete('connection');

    console.log(response.status, request.url, '(direct proxy)');
    return new Response(response.body, { headers });
  }

  // API paths: verify token service token and use webhook auth to internal routes
  const token = request.headers.get('authorization')?.split(' ')[1]
  if (!token) {
    return new Response('Unauthorized: No token provided', { status: 401 })
  }

  
  // Verify token against TOKEN SERVICE and extract user context
  let userId, userEmail, orgId;
  
  // Check cache first
  const cached = tokenCache.get(token);
  if (cached) {
    userId = cached.userId;
    userEmail = cached.userEmail;
    orgId = cached.orgId;
    console.log(`✅ Token cache hit for ${userId}`);
  } else {
    // Cache miss - verify token
    try {
      const startVerify = Date.now();
      
      const verifyResponse = await fetch(`${process.env.TOKENS_SERVICE_BACKEND_URL}/api/v1/tokens/verify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token }),
      });
      
      if (!verifyResponse.ok) {
        console.error('Token verification failed:', verifyResponse.status);
        return new Response('Unauthorized: Invalid token', { status: 401 })
      }
      
      const tokenInfo = await verifyResponse.json();
      const verifyTime = Date.now() - startVerify;
      
      if (!tokenInfo.valid) {
        return new Response('Unauthorized: Invalid token', { status: 401 })
      }
      
      // Extract user info from token service response
      const tokenData = tokenInfo.token || {};
      userId = tokenData.user_id || tokenInfo.user_id || 'anonymous';
      orgId = tokenData.org_id || tokenInfo.org_id || 'default';
      userEmail = tokenData.email || tokenInfo.email || '';
      
      // Only fetch email if not in token AND if we need it
      if (!userEmail) {
        try {
          const startEmail = Date.now();
          const emailResponse = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/users/${userId}`, {
            headers: {
              'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
              'X-Org-ID': orgId,
              'X-User-ID': userId,
              'X-Email': '',
            },
          });
          
          if (emailResponse.ok) {
            const userData = await emailResponse.json();
            userEmail = userData.email || '';
            console.log(`Email lookup: ${Date.now() - startEmail}ms`);
          } else {
            userEmail = '';
          }
        } catch (error) {
          console.error('Error fetching user email:', error);
          userEmail = '';
        }
      }
      
      // Cache the verified token
      tokenCache.set(token, userId, userEmail, orgId);
      console.log(`❌ Token cache miss - verified in ${verifyTime}ms, user: ${userId}, org: ${orgId}`);
    } catch (error) {
      console.error('Error verifying token:', error);
      return new Response('Unauthorized: Token verification failed', { status: 401 })
    }
  }

  // Use webhook auth to forward to internal TFE routes
  const webhookSecret = process.env.STATESMAN_BACKEND_WEBHOOK_SECRET;
  if (!webhookSecret) {
    console.error('STATESMAN_BACKEND_WEBHOOK_SECRET not configured');
    return new Response('Internal configuration error', { status: 500 });
  }

  const outgoingHeaders = new Headers();
  outgoingHeaders.set('Authorization', `Bearer ${webhookSecret}`);
  outgoingHeaders.set('X-User-ID', userId);
  outgoingHeaders.set('X-Email', userEmail);
  outgoingHeaders.set('X-Org-ID', orgId);
  
  const originalHost = request.headers.get('host') ?? '';
  if (originalHost) outgoingHeaders.set('x-forwarded-host', originalHost);
  outgoingHeaders.set('x-forwarded-proto', url.protocol.replace(':', ''));
  if (url.port) outgoingHeaders.set('x-forwarded-port', url.port);
  
  // Copy other relevant headers - INCLUDE accept-encoding for compression!
  // Without accept-encoding, backend sends uncompressed data (5-10x larger = slow)
  const headersToForward = ['content-type', 'accept', 'user-agent', 'accept-encoding'];
  headersToForward.forEach(h => {
    const value = request.headers.get(h);
    if (value) outgoingHeaders.set(h, value);
  });

  // Forward to internal TFE routes with webhook auth
  const startProxy = Date.now();
  const internalPath = url.pathname.replace('/tfe/api/v2', '/internal/tfe/api/v2');
  const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}${internalPath}${url.search}`, {
    method: request.method,
    headers: outgoingHeaders,
    body: request.method !== 'GET' && request.method !== 'HEAD' ? await request.blob() : undefined
  });

  const proxyTime = Date.now() - startProxy;

  // important, remove all encoding headers since the fetch already decompresses the gzip
  // the removal of headers avoids gzip errors in the client (double decompression)
  const headers = new Headers(response.headers);
  const wasCompressed = headers.get('Content-Encoding') === 'gzip';
  const contentLength = headers.get('content-length');
  
  headers.delete('Content-Encoding');
  headers.delete('content-length');
  headers.delete('transfer-encoding');
  headers.delete('connection');

  const compressionInfo = wasCompressed ? ` (gzip, ${contentLength} bytes)` : '';
  console.log(`${response.status} ${request.method} ${url.pathname} - Backend: ${proxyTime}ms${compressionInfo}`);
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