import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/_orchestrator/orgs/$orgId/access_policy')({
  server: {
    handlers: {
      GET: async ({ request, params }) => {
        try {
          
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/orgs/${params.orgId}/access_policy`, {
            method: 'GET',
            headers: request.headers,
          });
          return response
        } catch (error) {
          console.error('Error in PUT handler:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});