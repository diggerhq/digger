// Helper to generate request IDs for tracing
function generateRequestId(): string {
    return `ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export async function fetchProject(projectId: string, organizationId: string, userId: string) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/projects/${projectId}/`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
        'DIGGER_ORG_ID': organizationId,
        'DIGGER_USER_ID': userId,
        'DIGGER_ORG_SOURCE': 'workos',
        'X-Request-ID': generateRequestId(),
      },
    });
  
    if (!response.ok) {
      throw new Error(`Failed to fetch project: ${response.statusText}`);
    }
  
    return response.json();
}


export async function fetchProjects(organizationId: string, userId: string) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/api/projects/`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`,
        'DIGGER_ORG_ID': organizationId,
        'DIGGER_USER_ID': userId,
        'DIGGER_ORG_SOURCE': 'workos',
        'X-Request-ID': generateRequestId(),
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
            'X-Request-ID': generateRequestId(),
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