import { createFileRoute } from '@tanstack/react-router'
import { decodeJwt } from 'jose'
import { getWorkOS } from '@/authkit/ssr/workos'
import { getSessionFromCookie, saveSession } from '@/authkit/ssr/session'
import type { AccessToken } from '@workos-inc/node'

export const Route = createFileRoute('/api/auth/workos/switch-org')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        try {
          const { organizationId, pathname } = await request.json()

          if (!organizationId) {
            return new Response(JSON.stringify({ error: 'Missing organizationId' }), { status: 400 })
          }

          // Refresh/attach session for the target org
          const session = await getSessionFromCookie()
          if (!session) {
            return new Response(JSON.stringify({ error: 'Not authenticated' }), { status: 401 })
          }

          const { org_id: currentOrgId } = decodeJwt<AccessToken>(session.accessToken)

          try {
            const refreshResult = await getWorkOS().userManagement.authenticateWithRefreshToken({
              clientId: process.env.WORKOS_CLIENT_ID!,
              refreshToken: session.refreshToken,
              organizationId: organizationId ?? currentOrgId,
            })

            await saveSession(refreshResult)
          } catch (err: any) {
            const code = err?.error
            if (code === 'sso_required' || code === 'mfa_enrollment') {
              const url = getWorkOS().userManagement.getAuthorizationUrl({
                organizationId,
                clientId: process.env.WORKOS_CLIENT_ID!,
                provider: 'authkit',
                redirectUri: process.env.WORKOS_REDIRECT_URI!,
              })
              return new Response(JSON.stringify({ redirectUrl: url }), { status: 200 })
            }
            throw err
          }

          const to = pathname || '/'
          return new Response(JSON.stringify({ redirectUrl: to }), { status: 200 })
        } catch (err: any) {
          console.error('switch-org route error:', err)
          return new Response(JSON.stringify({ error: err?.message ?? 'Internal error' }), { status: 500 })
        }


      },
    },
  },
})


