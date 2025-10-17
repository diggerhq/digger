

export async function syncOrgToStatesman(orgId: string, orgName: string, userId: string, adminEmail: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/orgs`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-User-Email': adminEmail,
        },
        body: JSON.stringify({ 
            "org_id": orgId,
            "name": orgName,
            "created_by": adminEmail,
        })
    })  

    if (!response.ok) {
        throw new Error(`Failed to sync user: ${response.statusText}`);
    }

    return response.json();
}