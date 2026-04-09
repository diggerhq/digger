import { createFileRoute } from '@tanstack/react-router';
import { getConfig } from '../../../authkit/ssr/config';
import { saveSession } from '../../../authkit/ssr/session';
import { getWorkOS } from '../../../authkit/ssr/workos';
import { redirectWithFallback, errorResponseWithFallback } from '../../../api/helpers';
import { decodeJwt } from 'jose';
import type { AccessToken } from '@workos-inc/node';
import { syncOrgToBackend } from '@/api/orchestrator_orgs';
import { syncOrgToStatesman } from '@/api/statesman_orgs';
import { syncUserToStatesman } from '@/api/statesman_users';

export const Route = createFileRoute('/api/auth/callback')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const url = new URL(request.url);
        const code = url.searchParams.get('code');
        const state = url.searchParams.get('state');
        let returnPathname = state && state !== 'null' ? JSON.parse(atob(state)).returnPathname : null;

        if (code) {
          try {
            // Use the code returned to us by AuthKit and authenticate the user with WorkOS
            const { accessToken, refreshToken, user, impersonator } =
              await getWorkOS().userManagement.authenticateWithCode({
                clientId: getConfig('clientId'),
                code,
              });

            // If baseURL is provided, use it instead of request.nextUrl
            // This is useful if the app is being run in a container like docker where
            // the hostname can be different from the one in the request
            const url = new URL(request.url);

            // Cleanup params
            url.searchParams.delete('code');
            url.searchParams.delete('state');

            // Redirect to the requested path and store the session
            returnPathname = returnPathname ?? '/';

            // Extract the search params if they are present
            if (returnPathname.includes('?')) {
              const newUrl = new URL(returnPathname, 'https://example.com');
              url.pathname = newUrl.pathname;

              for (const [key, value] of newUrl.searchParams) {
                url.searchParams.append(key, value);
              }
            } else {
              url.pathname = returnPathname;
            }

            const response = redirectWithFallback(url.toString());

            if (!accessToken || !refreshToken) throw new Error('response is missing tokens');

            await saveSession({ accessToken, refreshToken, user, impersonator });

            // Ensure the signed-in WorkOS organization exists in dependent services.
            // This is a best-effort, login-time sync (in addition to any webhook-based
            // syncing) to reduce races/missed events; otherwise subsequent calls to
            // internal APIs can fail with "organization not found" for newly-seen orgs.
            const { org_id: organizationId } = decodeJwt<AccessToken>(accessToken);
            if (organizationId) {
              const organization = await getWorkOS().organizations.getOrganization(organizationId);
              await syncOrgToBackend(organization.id, organization.name, user.email);
              await syncOrgToStatesman(organization.id, organization.name, organization.name, user.id, user.email);
              await syncUserToStatesman(user.id, user.email, organization.id);
            }

            return response;
          } catch (error) {
            const errorRes = {
              error: error instanceof Error ? error.message : String(error),
            };

            console.error(errorRes);

            return errorResponse();
          }
        }

        return errorResponse();

        function errorResponse() {
          return errorResponseWithFallback({
            error: {
              message: 'Something went wrong',
              description:
                "Couldn't sign in. If you are not sure what happened, please contact your organization admin.",
            },
          });
        }
      },
    },
  },
});
