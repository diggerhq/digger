// src/routes/api/auth/workos/switch-org.ts
import { NoUserInfo, UserInfo } from '@/authkit/ssr/interfaces'
import { getSessionFromCookie, saveSession } from '@/authkit/ssr/session'
import { createFileRoute } from '@tanstack/react-router'
import { getSession } from '@tanstack/react-start/server'
import { getWorkOS } from '@/authkit/ssr/workos';

import { decodeJwt } from 'jose'
import { AccessToken } from '@workos-inc/node'

// WorkOS Node SDK must stay server-only.
import { createRequire } from 'node:module'
import { getConfig } from '@/authkit/ssr/config';
const require = createRequire(import.meta.url)
const { WorkOS,  } = require('@workos-inc/node')
const workos = new WorkOS(process.env.WORKOS_API_KEY!)

async function refreshSession(options: { organizationId?: string; ensureSignedIn: true }): Promise<UserInfo>;
async function refreshSession(options?: {
  organizationId?: string;
  ensureSignedIn?: boolean;
}): Promise<UserInfo | NoUserInfo>;
async function refreshSession({
  organizationId: nextOrganizationId,
  ensureSignedIn = false,
}: {
  organizationId?: string;
  ensureSignedIn?: boolean;
} = {}): Promise<UserInfo | NoUserInfo> {
  const session = await getSessionFromCookie();
  if (!session) {
    if (ensureSignedIn) {
      // await redirectToSignIn();
    }
    return { user: null };
  }

  const WORKOS_CLIENT_ID = getConfig('clientId');
  const WORKOS_REDIRECT_URI = getConfig('redirectUri');     

  const { org_id: organizationIdFromAccessToken } = decodeJwt<AccessToken>(session.accessToken);

  let refreshResult;

  try {
    refreshResult = getWorkOS().userManagement.authenticateWithRefreshToken({
      clientId: WORKOS_CLIENT_ID,
      refreshToken: session.refreshToken,
      organizationId: nextOrganizationId ?? organizationIdFromAccessToken,
    });
  } catch (error) {
    throw new Error(`Failed to refresh session: ${error instanceof Error ? error.message : String(error)}`, {
      cause: error,
    });
  }

  const headersList = new Headers();
  const url = headersList.get('x-url');

  await saveSession(refreshResult);

  const { accessToken, user, impersonator } = refreshResult;

  const {
    sid: sessionId,
    org_id: organizationId,
    role,
    roles,
    permissions,
    entitlements,
  } = decodeJwt<AccessToken>(accessToken);

  return {
    sessionId,
    user,
    organizationId,
    role,
    permissions,
    entitlements,
    impersonator,
    accessToken,
  };
}


export const Route = createFileRoute('/api/auth/workos/switch-org')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        try {
          const { organizationId, pathname } = await request.json() as {
            organizationId?: string
            pathname?: string
          }

          if (!organizationId) {
            return jsonError(400, 'Missing organizationId')
          }

          // 1) Refresh/attach session for the target org
          try {
            await refreshSession({ organizationId, ensureSignedIn: true })
          } catch (err: any) {
            // 2) Handle AuthKit redirect hints (AuthN/SSO/MFA)
            const authkitRedirect = err?.rawData?.authkit_redirect_url
            if (authkitRedirect) {
              return redirectResponse(authkitRedirect)
            }

            // 3) Handle SSO required / MFA enrollment by initiating authorization
            const code = err?.error
            if (code === 'sso_required' || code === 'mfa_enrollment') {
              const url = workos.userManagement.getAuthorizationUrl({
                organizationId,
                clientId: process.env.WORKOS_CLIENT_ID!,
                provider: 'authkit',
                redirectUri: process.env.WORKOS_REDIRECT_URI!,
              })
              return redirectResponse(url)
            }

            // Unknown error — bubble as 500
            throw err
          }

          // 4) Redirect back to the requested page after session switch
          const to = pathname || '/'
          // (No next/cache in TanStack — rely on loader invalidation on the client if needed)
          return redirectResponse(to)
        } catch (err: any) {
          console.error('switch-org error:', err)
          return jsonError(500, err?.message ?? 'Internal error')
        }
      },
    },
  },
})

function redirectResponse(location: string) {
  return new Response(null, {
    status: 302,
    headers: { Location: location },
  })
}

function jsonError(status: number, message: string) {
  return new Response(JSON.stringify({ error: message }), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
