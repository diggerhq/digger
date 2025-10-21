import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/_orchestrator/orgs/$orgId/plan_policy')({
  server: {
    handlers: {
      GET: async ({ request, params }) => {
        try {
          
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/orgs/${params.orgId}/plan_policy`, {
            method: 'GET',
            headers: request.headers,
          }).then(response => response.json())
          return response
        } catch (error) {
          console.error('Error in PUT handler:', error);
          return { error: 'Internal server error' }
        }
      },
    },
  },
});