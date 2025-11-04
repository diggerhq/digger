export async function listUnits(orgId: string, userId: string, email: string) {
    const startFetch = Date.now();
    
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

    const fetchTime = Date.now() - startFetch;
    console.log(`  → Statesman listUnits API call took ${fetchTime}ms (status: ${response.status})`);

    if (!response.ok) {
        throw new Error(`Failed to list units: ${response.statusText}`);
    }
    
    const parseStart = Date.now();
    const result = await response.json();
    const parseTime = Date.now() - parseStart;
    
    if (parseTime > 100) {
        console.log(`  → JSON parsing took ${parseTime}ms`);
    }
    
    return result;
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

export async function forcePushState(orgId: string, userId: string, email: string, unitId: string, state: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units/${unitId}/upload`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
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
        },
    });
    if (!response.ok) {
        throw new Error(`Failed to get unit status: ${response.statusText}`);
    }
    return response.json();
}

export async function createUnit(orgId: string, userId: string, email: string, name: string, requestId?: string) {
    const rid = requestId || `unit-${Date.now()}-api`
    const startApi = Date.now();
    
    console.log(`[${rid}] 🌐 API_LAYER: Making HTTP POST to Statesman backend`);

    const fetchStart = Date.now();
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/units`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': email,
            'X-Request-ID': rid, // Pass request ID to backend for correlation
        },
        body: JSON.stringify({
            name: name,
        }),
    });

    const fetchTime = Date.now() - fetchStart;
    const statusEmoji = response.ok ? '✅' : '❌';
    console.log(`[${rid}] ${statusEmoji} API_LAYER: Statesman responded - status: ${response.status}, time: ${fetchTime}ms`);

    if (!response.ok) {
        const errorBody = await response.text().catch(() => 'Unknown error');
        console.error(`[${rid}] ❌ API_LAYER: Backend error - ${response.status} ${response.statusText}: ${errorBody}`);
        throw new Error(`Failed to create unit: ${response.statusText}`);
    }

    const parseStart = Date.now();
    const result = await response.json();
    const parseTime = Date.now() - parseStart;

    const totalApiTime = Date.now() - startApi;
    
    if (parseTime > 100) {
        console.log(`[${rid}] 📄 API_LAYER: JSON parse took ${parseTime}ms`);
    }
    
    console.log(`[${rid}] ✅ API_LAYER: Complete - total: ${totalApiTime}ms (fetch: ${fetchTime}ms, parse: ${parseTime}ms)`);

    return result;
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