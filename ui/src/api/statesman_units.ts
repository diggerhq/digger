// Helper to generate request IDs for tracing
function generateRequestId(): string {
    return `ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export async function listUnits(orgId: string, userId: string, email: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units`, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },  
    });

    if (!response.ok) {
        throw new Error(`Failed to list units: ${response.statusText}`);
    }
    
    return response.json();
}

export async function getUnit(orgId: string, userId: string, email: string, unitId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}`, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to get unit: ${response.statusText}`);
    }
    return response.json();
}

export async function getUnitVersions(orgId: string, userId: string, email: string, unitId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/versions`, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to get unit: ${response.statusText}`);
    }
    return response.json();
}


export async function lockUnit(orgId: string, userId: string, email: string, unitId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/lock`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to lock unit: ${response.statusText}`);
    }
    return response.json();
}

export async function unlockUnit(orgId: string, userId: string, email: string, unitId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/unlock`, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
    });     
    if (!response.ok) {
        throw new Error(`Failed to unlock unit: ${response.statusText}`);
    }
    return response.json();
}

export async function forcePushState(orgId: string, userId: string, email: string, unitId: string, state: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/upload`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
        body: state,
    });
    if (!response.ok) {
        throw new Error(`Failed to force push state: ${response.statusText}`);
    }
    return response.json();
}

export async function downloadLatestState(orgId: string, userId: string, email: string, unitId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/download`, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
    });
    return response.json()
}

export async function restoreUnitStateVersion(orgId: string, userId: string, email: string, unitId: string, timestamp: string, lockId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/restore`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
        body: JSON.stringify({
            timestamp: timestamp,
            lock_id: lockId,
        }),
    });
    if (!response.ok) {
        throw new Error(`Failed to restore unit state version: ${response.statusText}`);
    }
    return response.json();
}

export async function getUnitStatus(orgId: string, userId: string, email: string, unitId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/status`, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to get unit status: ${response.statusText}`);
    }
    return response.json();
}

export async function createUnit(
    orgId: string, 
    userId: string, 
    email: string, 
    name: string,
    tfeAutoApply?: boolean,
    tfeExecutionMode?: string,
    tfeTerraformVersion?: string,
    tfeEngine?: string,
    tfeWorkingDirectory?: string
) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
        body: JSON.stringify({
            name: name,
            tfe_auto_apply: tfeAutoApply,
            tfe_execution_mode: tfeExecutionMode,
            tfe_terraform_version: tfeTerraformVersion,
            tfe_engine: tfeEngine,
            tfe_working_directory: tfeWorkingDirectory,
        }),
    });

    if (!response.ok) {
        throw new Error(`Failed to create unit: ${response.statusText}`);
    }

    return response.json();
}

export async function updateUnit(
    orgId: string, 
    userId: string, 
    email: string, 
    unitId: string,
    tfeAutoApply?: boolean,
    tfeExecutionMode?: string,
    tfeTerraformVersion?: string,
    tfeEngine?: string,
    tfeWorkingDirectory?: string
) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}`, {
        method: 'PATCH',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': generateRequestId(),
        },
        body: JSON.stringify({
            tfe_auto_apply: tfeAutoApply,
            tfe_execution_mode: tfeExecutionMode,
            tfe_terraform_version: tfeTerraformVersion,
            tfe_engine: tfeEngine,
            tfe_working_directory: tfeWorkingDirectory,
        }),
    });

    if (!response.ok) {
        throw new Error(`Failed to update unit: ${response.statusText}`);
    }

    return response.json();
}

export async function deleteUnit(orgId: string, userId: string, email: string, unitId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}`, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Request-ID': generateRequestId(),
            'X-Email': email,
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to delete unit: ${response.statusText}`);
    }

}