import { createFileRoute } from '@tanstack/react-router';
import { WorkOS , Event as WorkOsEvent } from '@workos-inc/node';
import { syncOrgToBackend } from '@/api/orchestrator_orgs';
import { syncUserToBackend } from '@/api/orchestrator_users';
import { createOrgForUser, listUserOrganizationInvitations, getOrganisationDetails } from '@/authkit/ssr/workos_api';

type WorkosUserCreatedEvent = WorkOsEvent

export const Route = createFileRoute('/api/auth/workos/webhooks')({
  server: {
    handlers: {
        POST: async ({ request }) => {
          const event: WorkosUserCreatedEvent = await request.json();
          const sigHeader = request.headers.get("workos-signature");
          const workos = new WorkOS(process.env.WORKOS_API_KEY);
      
          await workos.webhooks.constructEvent({
            payload: event,
            sigHeader: sigHeader!,
            secret: process.env.WORKOS_WEBHOOK_SECRET!,
          });

          
          if (event.event === "user.created") {
            console.log("Creating personal organization for the user", event.data.email)
            const orgDetails = await createOrgForUser(event.data.id, "Personal");
            let orgName = orgDetails.name;
            let orgId = orgDetails.id;
            console.log(`User ${event.data.email} is not invited to an organization, creating a new one`);
      
            try {
              console.log("Syncing organization to backend orgName", orgName, "orgId", orgId);
              await syncOrgToBackend(orgId, orgName, null);
            } catch (error) {
              console.error(`Error syncing organization to backend:`, error);
              throw error;
            }
      
            // == syncing invitations == 
            let userInvitations;
      
            try {
              userInvitations = await listUserOrganizationInvitations(event.data.email)
            } catch (error) {
              console.error('Error fetching user invitations:', error);
              throw error;
            }
      
            if (userInvitations.length > 0) {
              for (const invitation of userInvitations) {
                let orgId = invitation.organizationId!;
                const orgDetails = await getOrganisationDetails(orgId);
                let orgName = orgDetails.name;
                try {
                  console.log("Syncing organization to backend orgName", orgName, "orgId", orgId);
                  await syncOrgToBackend(orgId, orgName, event.data.email);
                } catch (error) {
                  console.error(`Error syncing organization to backend:`, error);
                  throw error;
                }
          
                try {
                  await syncUserToBackend(event.data.id, event.data.email, orgId);
                } catch (error) {
                  console.error(`Error syncing user to backend:`, error);
                  throw error;
                }   
              }
            }      
          }
      
          return new Response('Webhook received', { status: 200 });          
        },
      },
    },
});
