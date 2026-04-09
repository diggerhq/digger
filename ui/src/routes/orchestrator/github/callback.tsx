import { createFileRoute } from '@tanstack/react-router';
import { sendGithubInstallationEvent } from '../../../lib/analytics';
import { getOrchestratorBackendUrl, requireUiAuth } from './proxyUtils';

export const Route = createFileRoute('/orchestrator/github/callback')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        try {
          const authResult = await requireUiAuth(request);
          if (authResult instanceof Response) return authResult;
          const { user, organisationId } = authResult;

          const requestUrl = new URL(request.url);
          const installationId = requestUrl.searchParams.get('installation_id');
          await sendGithubInstallationEvent(
            installationId || 'unknown',
            user,
            organisationId || 'unknown',
          );

          const backendUrl = getOrchestratorBackendUrl();
          const postResponse = await fetch(`${backendUrl}/api/github/link`, {
            method: 'POST',
            headers: {
              Authorization: `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
              DIGGER_ORG_ID: organisationId || '',
              DIGGER_ORG_SOURCE: 'workos',
              DIGGER_USER_ID: user.id,
            },
            body: JSON.stringify({ installation_id: installationId }),
          });

          if (!postResponse.ok) {
            return new Response(
              'Unexpected error while processing GitHub installation',
              { status: postResponse.status },
            );
          }

          return await fetch(
            `${backendUrl}/github/callback${requestUrl.search}`,
            {
              method: request.method,
              headers: request.headers,
              body: request.method !== 'GET' ? await request.blob() : undefined,
            },
          );
        } catch (error) {
          console.error('Error in GitHub callback:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});
