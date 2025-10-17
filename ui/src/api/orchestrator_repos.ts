export async function syncOrgToBackend(orgId: string, orgName: string, adminEmail: string | null) {
  const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/_internal/api/upsert_org`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`
    },
    body: JSON.stringify({ 
        org_name: orgName,
        external_id: orgId,
        external_source: "workos",
        admin_email: adminEmail || null,
    })
  });

  if (!response.ok) {
    throw new Error(`Failed to sync organization: ${response.statusText}`);
  }

  return response.json();
}

export async function syncUserToBackend(userId: string, userEmail: string, orgId: string) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/_internal/api/create_user`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`
        },
        body: JSON.stringify({ 
            external_id: userId,
            external_source: "workos",
            external_org_id: orgId,
            email: userEmail,
        })
    })  

    if (!response.ok) {
        throw new Error(`Failed to sync user: ${response.statusText}`);
    }

    return response.json();
}

export async function fetchRepos(organizationId: string, userId: string) {
  const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/repos/`, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
      'DIGGER_ORG_ID': organizationId,
      'DIGGER_USER_ID': userId,
      'DIGGER_ORG_SOURCE': 'workos',
    },
  });

  if (!response.ok) {
    throw new Error(`Failed to fetch repos: ${response.statusText}`);
  }

  return response.json();
}

export async function fetchRepoDetails(repoId: string, organisationId: string, userId: string) {
  const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/repos/${repoId}`, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
      'DIGGER_ORG_ID': organisationId,
      'DIGGER_USER_ID': userId,
      'DIGGER_ORG_SOURCE': 'workos',
    },
  });
}


