

export async function syncUserToStatesman(userId: string, userEmail: string, orgId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/users`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-User-Email': userEmail,
        },
        body: JSON.stringify({ 
            subject: userId,
            email: userEmail,
        })
    })  

    if (!response.ok) {
        throw new Error(`Failed to sync user: ${response.statusText}`);
    }

    return response.json();
}