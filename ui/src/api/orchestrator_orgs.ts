
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

export async function getOrgSettings(
    organizationId: string | null,
    userId: string | null
  ) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/orgs/settings`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
        'Content-Type': 'application/json',
        'DIGGER_ORG_ID': organizationId || '',
        'DIGGER_USER_ID': userId || '',
        'DIGGER_ORG_SOURCE': 'workos'
      }
    })
  
    if (response.status === 404) {
      return {
        drift_webhook_url: "",
        drift_enabled: false,
        drift_cron_tab: "0 9 * * *"  // Default to daily at 9 AM
      }
    }
  
    if (!response.ok) {
      const text = await response.text()
      console.log(text)
      throw new Error('Failed to get organization settings')
    }
  
    return response.json()
  }

export async function updateOrgSettings(
    organizationId: string | null,
    userId: string | null,
    settings: {
      drift_webhook_url?: string,
      drift_enabled?: boolean,
      drift_cron_tab?: string,
      billing_plan?: string,
      billing_stripe_subscription_id?: string,
      slack_channel_name?: string,
    }
  ) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/orgs/settings`, {
      method: 'PUT',
      headers: {
        'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
        'Content-Type': 'application/json',
        'DIGGER_ORG_ID': organizationId || '',
        'DIGGER_USER_ID': userId || '',
        'DIGGER_ORG_SOURCE': 'workos'
      },
      body: JSON.stringify(settings)
    })
  
    if (!response.ok) {
      const text = await response.text()
      console.log(text)
      throw new Error('Failed to update organization settings')
    }
  
    return response.json()
  }
