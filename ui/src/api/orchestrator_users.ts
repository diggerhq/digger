

export async function syncUserToBackend(userId: string, userEmail: string, orgId: string) {
    const response = await fetch(`${process.env.ORCHESTRATOR_BACKEND_URL}/_internal/api/create_user`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.ORCHESTRATOR_BACKEND_SECRET}`
        },
        body: JSON.stringify({ 
            external_id: userId,
            external_source: "workos",
            external_org_id: orgId,
            email: userEmail,
        })
    })  

    if (response.status === 409) {
        console.log("User already exists in orchestrator")
        return response.json();
    }

    if (!response.ok) {
        throw new Error(`Failed to sync user: ${response.statusText}`);
    }

    return response.json();
}