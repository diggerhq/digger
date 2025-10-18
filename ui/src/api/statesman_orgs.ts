

export async function syncOrgToStatesman(orgId: string, orgName: string, userId: string, adminEmail: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/orgs`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': adminEmail,
        },
        body: JSON.stringify({ 
            "org_id": orgId,
            "name": orgName,
            "created_by": adminEmail,
        })
    })  

    console.log(orgId)
    console.log(orgName)
    console.log(userId)
    console.log(adminEmail)

    if (!response.ok) {
        throw new Error(`Failed to sync organization to statesman: ${response.statusText}`);
    }

    return response.json();
}