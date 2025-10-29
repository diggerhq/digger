import { createFileRoute, createRoute } from '@tanstack/react-router'
import { Route as rootRoute } from '@/routes/__root'

export const terraformRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: '/.well-known/terraform.json',
    // component: () => <pre>{JSON.stringify({ backend: 's3' })}</pre>,
    server: {
        handlers: {
            GET: async ({ request }) => {
                const payload = {
                    "modules.v1":"/v1/modules/",
                    "motd.v1":"/tfe/api/v2/motd",
                    "state.v2":"/tfe/api/v2/",
                    "tfe.v2":"/tfe/api/v2/",
                    "tfe.v2.1":"/tfe/api/v2/",
                    "tfe.v2.2":"/tfe/api/v2/"
                }
                return new Response(JSON.stringify(payload), { status: 200, headers: { 'Content-Type': 'application/json' } });
            }
        }
    }
  })