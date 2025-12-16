import { createFileRoute } from '@tanstack/react-router';


export const Route = createFileRoute('/_orchestrator/repos/$namespace/projects/$projectName/jobs/$jobId/set-status')({
  server: {
    handlers: {
      POST: async ({ request, params }) => {
        try {

          // Clone headers and strip problematic ones
          const headers = new Headers(request.headers);
          headers.delete('content-length');
          headers.delete('transfer-encoding');

          const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/repos/${params.namespace}/projects/${params.projectName}/jobs/${params.jobId}/set-status`, {
            method: 'POST',
            headers: headers,
            body: request.body,
            // @ts-expect-error: 'duplex' is required by Node/undici for streaming bodies
            duplex: 'half',            
          });

          return response;
        } catch (error) {
          console.error('Error in POST handler:', error);
          return new Response('Internal server error', { status: 500 });
        }
      },
    },
  },
});