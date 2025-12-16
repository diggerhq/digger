import { unsealData } from 'iron-session';
import { decodeJwt } from 'jose';

// Request logging utilities
export async function extractUserInfoFromRequest(req) {
  try {
    const cookieName = process.env.WORKOS_COOKIE_NAME || 'wos-session';
    const cookiePassword = process.env.WORKOS_COOKIE_PASSWORD;
    
    if (!cookiePassword) {
      return { userId: 'anonymous', orgId: 'anonymous' };
    }
    
    const cookieHeader = req.headers?.cookie || req.getHeader?.('cookie');
    if (!cookieHeader) {
      return { userId: 'anonymous', orgId: 'anonymous' };
    }
    
    const cookies = cookieHeader.split(';').reduce((acc, cookie) => {
      const [key, value] = cookie.trim().split('=');
      acc[key] = decodeURIComponent(value);
      return acc;
    }, {});
    
    const sessionCookie = cookies[cookieName];
    if (!sessionCookie) {
      return { userId: 'anonymous', orgId: 'anonymous' };
    }
    
    const session = await unsealData(sessionCookie, {
      password: cookiePassword,
    });
    
    if (!session?.user?.id || !session?.accessToken) {
      return { userId: 'anonymous', orgId: 'anonymous' };
    }
    
    // Decode JWT to get organization ID
    let orgId = 'anonymous';
    try {
      const decoded = decodeJwt(session.accessToken);
      orgId = decoded.org_id || 'anonymous';
    } catch (error) {
      // If JWT decode fails, just use anonymous
    }
    
    return { userId: session.user.id, orgId };
  } catch (error) {
    return { userId: 'anonymous', orgId: 'anonymous' };
  }
}

export function logRequestInit(method, path, requestId, userId, orgId) {
  console.log(JSON.stringify({
    event: 'request_initialized',
    method,
    path,
    requestId,
    userId,
    orgId,
  }));
}

export function logResponse(method, path, requestId, latency, statusCode, commitSha) {
  console.log(JSON.stringify({
    event: 'response_sent',
    method,
    path,
    requestId,
    latency,
    statusCode,
    commitSha: commitSha || 'unknown',
  }));
}

