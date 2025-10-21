import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/_orchestrator/job_artefacts')({
  server: {
    handlers: {
      PUT: async ({ request }) => {
        try {
          const body = await request.json();
          
          const response = await fetch(`${process.env.BACKEND_URL}/job_artefacts`, {
            method: 'PUT',
            headers: {
              'Authorization': `Bearer ${process.env.BACKEND_SECRET}`,
              'Content-Type': 'application/json'
            },
            body: JSON.stringify(body)
          });

          if (!response.ok) {
            const errorText = await response.text();
            console.error('Backend request failed:', errorText);
            return new Response(
              JSON.stringify({ error: 'Failed to proxy request to backend' }),
              { 
                status: response.status,
                headers: { 'Content-Type': 'application/json' }
              }
            );
          }

          const data = await response.json();
          return new Response(
            JSON.stringify(data),
            { 
              status: 200,
              headers: { 'Content-Type': 'application/json' }
            }
          );

        } catch (error) {
          console.error('Error in PUT handler:', error);
          return new Response(
            JSON.stringify({ error: 'Internal server error' }),
            { 
              status: 500,
              headers: { 'Content-Type': 'application/json' }
            }
          );
        }
      },
    },
  },
});