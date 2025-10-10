
export async function fetchProjects(organizationId: string, userId: string) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/projects/`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
        'DIGGER_ORG_ID': organizationId,
        'DIGGER_USER_ID': userId,
        'DIGGER_ORG_SOURCE': 'workos',
      },
    });
  
    if (!response.ok) {
      throw new Error(`Failed to fetch projects: ${response.statusText}`);
    }
  
    return response.json();
  } 

export async function updateProject(projectId: string, driftEnabled: boolean, organizationId: string, userId: string) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/projects/${projectId}/`, {
        method: 'PUT',
        headers: {
            'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
            'DIGGER_ORG_ID': organizationId,
            'DIGGER_USER_ID': userId,
            'DIGGER_ORG_SOURCE': 'workos',
        },
        body: JSON.stringify({
            drift_enabled: driftEnabled,
        }),
    });

    if (!response.ok) {
        throw new Error(`Failed to update project drift enabled: ${response.statusText}`);
    }

    return response.json();
}

export async function fetchBillingInfo(organizationId: string, userId: string) {
    try {
      const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/billing`, {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
          'DIGGER_ORG_ID': organizationId,
          'DIGGER_USER_ID': userId,
          'DIGGER_ORG_SOURCE': 'workos',
        },
      });
    
      if (response.status === 404) {
        // If no billing info exists, return default free plan
        return {
          billing_plan: "free",
          remaining_free_projects: 0,
          monitored_projects_limit: 3,
        };
      }

      if (!response.ok) {
        throw new Error(`Failed to fetch billing info: ${response.statusText}`);
      }
    
      return response.json();
    } catch (error) {
      // If backend is not available, return a mock plan for demonstration
      // You can change this to test different plan types
      throw error;
    }
}

export async function fetchProject(projectId: string, organizationId: string, userId: string) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/projects/${projectId}/`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
        'DIGGER_ORG_ID': organizationId,
        'DIGGER_USER_ID': userId,
        'DIGGER_ORG_SOURCE': 'workos',
      },
    });
  
    if (!response.ok) {
      throw new Error(`Failed to fetch project: ${response.statusText}`);
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
    const response = await fetch(`${process.env.ORCHESTRATOR_ORCHESTRATOR_BACKEND_URL}/api/orgs/settings`, {
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
