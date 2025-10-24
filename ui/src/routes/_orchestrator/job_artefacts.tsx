import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/_orchestrator/job_artefacts')({
  server: {
    handlers: {
      PUT: async ({ request }) => {
        try {
          const body = await request.json();
          
          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/job_artefacts`, {
            method: 'PUT',
            headers: request.headers,
            body: JSON.stringify(body)
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