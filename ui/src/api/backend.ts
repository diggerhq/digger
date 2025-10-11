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



export async function testSlackWebhook(
  notification_url: string
) {
  const response = await fetch(`${process.env.DRIFT_REPORTING_BACKEND_URL}/_internal/send_test_slack_notification_for_url`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${process.env.DRIFT_REPORTING_BACKEND_WEBHOOK_SECRET}`,
    },
    body: JSON.stringify({
      notification_url: notification_url
    })
  })

  console.log(response)
  if (!response.ok) {
    const text = await response.text()
    console.log(text)
    throw new Error('Failed to test Slack webhook')
  }

  return response.text()
} 