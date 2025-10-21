import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/_orchestrator/repos/$namespace/report-projects')({
  server: {
    handlers: {
      POST: async ({ request, params }) => {
        try {
          const body = await request.json();
          
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/repos/${params.namespace}/report-projects`, {
            method: 'POST',
            headers: request.headers,
            body: JSON.stringify(body)
          }).then(response => response.json())

          return response

        } catch (error) {
          console.error('Error in POST handler:', error);
          return { error: 'Internal server error' }
        }
      },
    },
  },
});