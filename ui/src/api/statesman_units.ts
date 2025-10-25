export async function listUnits(orgId: string, userId: string, email: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units`, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
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
        },
    });     
    if (!response.ok) {
        throw new Error(`Failed to unlock unit: ${response.statusText}`);
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
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to get unit status: ${response.statusText}`);
    }
    return response.json();
}

export async function createUnit(orgId: string, userId: string, email: string, name: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
        },
        body: JSON.stringify({
            name: name,
        }),
    });
    console.log(response)
    if (!response.ok) {
        throw new Error(`Failed to create unit: ${response.statusText}`);
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
            'X-Email': email,
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to delete unit: ${response.statusText}`);
    }

}