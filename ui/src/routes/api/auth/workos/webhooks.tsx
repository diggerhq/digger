import { createFileRoute } from '@tanstack/react-router';
import { WorkOS , Event as WorkOsEvent } from '@workos-inc/node';
import { syncOrgToBackend } from '@/api/orchestrator_orgs';
import { syncUserToBackend } from '@/api/orchestrator_users';
import { createOrgForUser, listUserOrganizationInvitations, getOrganisationDetails, getOranizationsForUser } from '@/authkit/ssr/workos_api';
import { syncOrgToStatesman } from '@/api/statesman_orgs';
import { syncUserToStatesman } from '@/api/statesman_users';

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
            const personalOrgDisplayName = "Personal"

            const userOrganizations = await getOranizationsForUser(event.data.id);
            const personalOrg  = userOrganizations.filter(membership => 
              membership.organizationName === personalOrgDisplayName
            );
            const personalOrgExists = personalOrg.length > 0;

            let orgName, orgId;
            if (personalOrgExists) {
              console.log(`User ${event.data.email} is already has a personal organization, using it`);
              orgName = personalOrg[0].organizationName;
              orgId = personalOrg[0].organizationId;
            } else {
              console.log(`User ${event.data.email} is not invited to an organization, creating a new one`);
              const orgDetails = await createOrgForUser(event.data.id, personalOrgDisplayName);
              orgName = orgDetails.name;
              orgId = orgDetails.id;  
            }
      
            try {
              console.log("Syncing organization to backend orgName", orgName, "orgId", orgId);
              await syncOrgToBackend(orgId, orgName, event.data.email);
              await syncOrgToStatesman(orgId, personalOrgDisplayName, personalOrgDisplayName, event.data.id, event.data.email);
            } catch (error) {
              console.error(`Error syncing organization to backend:`, error);
              throw error;
            }
      
            try {
              await syncUserToBackend(event.data.id!, event.data.email, orgId);
              await syncUserToStatesman(event.data.id!, event.data.email, orgId);
            } catch (error) {
              console.error(`Error syncing user to backend:`, error);
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
                  const orgDetails = await getOrganisationDetails(invitation.organizationId!);
                  let orgName = orgDetails.name;
                  console.log(`Syncing organization ${orgName} to backend and statesman`);
                  try {
                      await syncOrgToBackend(invitation.organizationId!, orgName, invitation.userEmail);
                      await syncOrgToStatesman(invitation.organizationId!, orgName, orgName, invitation.userId!, invitation.userEmail!);
                  } catch (error) {
                      console.error(`Error syncing organization to backend:`, error);
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

