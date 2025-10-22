import { createFileRoute } from '@tanstack/react-router';
import { WorkOS } from '@workos-inc/node';
import { sendGithubInstallationEvent } from "../../../lib/analytics-server";
import { getWorkOS } from '../../../authkit/ssr/workos';
import { getAuth } from '../../../authkit/serverFunctions';


export const Route = createFileRoute('/orchestrator/github/callback')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const workos = await getWorkOS();
        try {
          // Get user and org info from headers
          const { auth, organisationId } = await getAuth();
          const user = auth.user;
          const organizationId = organisationId;

          if (!user) {
            return new Response('Unauthorized', { status: 401 });
          }

          const requestUrl = new URL(request.url);
          const installationId = requestUrl.searchParams.get('installation_id');
          
        //   await sendGithubInstallationEvent(installationId || 'unknown', user, organizationId || 'unknown');
          
          // Forward to backend
          const postResponse = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/github/link`, {
            method: 'POST',
            headers: {
              'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
              'DIGGER_ORG_ID': organizationId || '',
              'DIGGER_ORG_SOURCE': 'workos', 
              'DIGGER_USER_ID': user.id,
            },
            body: JSON.stringify({
              installation_id: installationId
            })
          });

          if (!postResponse.ok) {
            return new Response('Unexpected error while processing GitHub installation', { 
              status: postResponse.status 
            });
          }

          // Forward the original request
          const url = new URL(request.url);
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/github/callback/${url.search}`, {
            method: request.method,
            headers: request.headers,
            body: request.method !== 'GET' ? await request.blob() : undefined
          });

          return response;

        } catch (error) {
          console.error('Error in GitHub callback:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});