import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/_orchestrator/repos/$namespace/projects/$projectName/access_policy')({
  server: {
    handlers: {
      GET: async ({ request, params }) => {
        try {
          
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/repos/${params.namespace}/projects/${params.projectName}/access_policy`, {
            method: 'GET',
            headers: request.headers,
          }).then(response => response.json())
          return response
        } catch (error) {
          console.error('Error in GET handler:', error);
          return { error: 'Internal server error' }
        }
      },
    },
  },
});