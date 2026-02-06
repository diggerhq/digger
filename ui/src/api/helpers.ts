import { withAuth, getSessionFromCookie, saveSession } from '@/authkit/ssr/session';
import { getWorkOS } from '@/authkit/ssr/workos';
import { getConfig } from '@/authkit/ssr/config';
import { decodeJwt } from 'jose';
import type { AccessToken } from '@workos-inc/node';

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

/**
 * Refreshes the session and returns the new organizationId if available.
 * This is used when the session doesn't have an org_id yet (e.g., webhook delay).
 */
async function refreshSessionForOrg(): Promise<string | undefined> {
  const session = await getSessionFromCookie();
  if (!session?.refreshToken) {
    return undefined;
  }

  try {
    const { accessToken, refreshToken, user, impersonator } =
      await getWorkOS().userManagement.authenticateWithRefreshToken({
        clientId: getConfig('clientId'),
        refreshToken: session.refreshToken,
      });

    await saveSession({ accessToken, refreshToken, user, impersonator });

    const { org_id: organizationId } = decodeJwt<AccessToken>(accessToken);
    return organizationId;
  } catch (error) {
    console.error('Failed to refresh session for org:', error);
    return undefined;
  }
}

function redirectWithFallback(redirectUri: string, headers?: Headers) {
    const newHeaders = headers ? new Headers(headers) : new Headers();
    newHeaders.set('Location', redirectUri);

    return new Response(null, { status: 307, headers: newHeaders });
  }

  function errorResponseWithFallback(errorBody: { error: { message: string; description: string } }) {
    return new Response(JSON.stringify(errorBody), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    });
  }

/**
 * Requires authentication and returns session-derived user identity.
 * Use this instead of accepting userId/organizationId from client input
 * to prevent IDOR vulnerabilities.
 *
 * If organizationId is missing (e.g., webhook hasn't processed yet),
 * this will retry by refreshing the session up to 3 times.
 */
export async function requireAuth() {
  const auth = await withAuth();
  if (!auth.user) {
    throw new Error('Unauthorized');
  }

  let organizationId = auth.organizationId;

  // If no organizationId, the webhook may not have processed yet.
  // Retry by refreshing the session to get updated JWT claims.
  if (!organizationId) {
    const MAX_RETRIES = 3;
    const RETRY_DELAY_MS = 1000;

    for (let attempt = 1; attempt <= MAX_RETRIES; attempt++) {
      console.log(`No organizationId in session, refreshing (attempt ${attempt}/${MAX_RETRIES})...`);

      organizationId = await refreshSessionForOrg();

      if (organizationId) {
        console.log(`Organization found after ${attempt} refresh attempt(s): ${organizationId}`);
        break;
      }

      if (attempt < MAX_RETRIES) {
        await sleep(RETRY_DELAY_MS);
      }
    }
  }

  if (!organizationId) {
    throw new Error('No organization available. Please try logging out and back in.');
  }

  return {
    userId: auth.user.id,
    email: auth.user.email!,
    organizationId,
  };
}

export { redirectWithFallback, errorResponseWithFallback };