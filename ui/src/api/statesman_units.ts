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