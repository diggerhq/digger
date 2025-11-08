import { syncOrgToBackend } from '@/api/orchestrator_orgs';
import { syncOrgToStatesman } from '@/api/statesman_orgs';
import { getWorkOS } from '@/authkit/ssr/workos';
import { createFileRoute } from '@tanstack/react-router'
import { syncUserToBackend } from '@/api/orchestrator_users';
import { syncUserToStatesman } from '@/api/statesman_users';

async function syncWorkosOrgs() {
    const workos = await getWorkOS()

    let after: string | undefined = undefined;
    let totalSynced = 0;

    do {
        const page = await workos.organizations.listOrganizations({ after });
        for (const org of page.data) {
            try {                
                // Determine oldest member's email to use as adminEmail
                let adminEmail: string | null = null;
                let oldestUserId: string | null = null;
                let orgMemberships: any[] = [];
                try {
                    // List memberships for this organization
                    const memberships = await getWorkOS().userManagement.listOrganizationMemberships({
                        organizationId: org.id,
                    } as any);
                    orgMemberships = (memberships as any)?.data ?? [];

                    const sorted = orgMemberships.slice?.().sort?.((a: any, b: any) => {
                        const aTime = new Date(a?.createdAt ?? a?.created_at ?? 0).getTime();
                        const bTime = new Date(b?.createdAt ?? b?.created_at ?? 0).getTime();
                        return aTime - bTime;
                    }) ?? [];
                    const oldest = sorted[0] as any;
                    if (oldest?.userId || oldest?.user?.id) {
                        const userId = oldest?.userId ?? oldest?.user?.id;
                        oldestUserId = userId ?? null;
                        try {
                            const user = await getWorkOS().userManagement.getUser(userId);
                            adminEmail = (user as any)?.email ?? null;
                        } catch {
                            adminEmail = (oldest as any)?.user?.email ?? null;
                        }
                    }
                } catch (err) {
                    console.warn('Could not resolve oldest org member for adminEmail', { orgId: org.id, err: err instanceof Error ? err.message : String(err) });
                }

                await syncOrgToBackend(org.id, org.name, adminEmail);
                await syncOrgToStatesman(org.id, org.name, org.name, oldestUserId, adminEmail);

                // After org sync completes, sync all users for this org to backend and statesman
                for (const m of orgMemberships) {
                    const uid = (m as any)?.userId ?? (m as any)?.user?.id;
                    if (!uid) continue;
                    let email = (m as any)?.user?.email ?? '';
                    if (!email) {
                        try {
                            const user = await getWorkOS().userManagement.getUser(uid);
                            email = (user as any)?.email ?? '';
                        } catch {}
                    }
                    if (!email || email === adminEmail) continue;
                    await Promise.allSettled([
                        syncUserToBackend(uid, email, org.id),
                        syncUserToStatesman(uid, email, org.id),
                    ]);
                }
                console.log("Synced organization to backend", { id: org.id, name: org.name });
                totalSynced += 1;
            } catch (error) {
                console.error("Failed to sync organization", { id: org.id, name: org.name, error: error instanceof Error ? error.message : String(error) });
            }
        }
        after = page.listMetadata?.after ?? undefined;
    } while (after);

    console.log(`Completed WorkOS organization resync`, { totalSynced });
}

export const Route = createFileRoute('/api/internal/sync-workos-orgs' as any)({
    server: {
      handlers: {
        POST: async ({ request }) => {
            const internalTasksSecret = request.headers.get('Authorization')?.replace('Bearer ', '');
            if (internalTasksSecret !== process.env.INTERNAL_TASKS_SECRET) {
                return new Response('Unauthorized', { status: 401 });
            }
            syncWorkosOrgs();
            return new Response('OK');
        }   
    },
  },
})

