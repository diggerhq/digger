

export async function syncOrgToStatesman(orgId: string, orgName: string, displayName: string, userId: string | null, adminEmail: string | null) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/orgs`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': "",
            'X-User-ID': userId ?? '',
            'X-Email': adminEmail ?? '',
        },
        body: JSON.stringify({ 
            "external_org_id": orgId,
            "name": orgName,
            "display_name": displayName,
        })
    })  

    if (response.status === 409) {
        console.log("Org already exists in statesman")
        return response.json();
    }

    if (!response.ok) {
        throw new Error(`Failed to sync organization to statesman: ${response.statusText}`);
    }

    return response.json();
}