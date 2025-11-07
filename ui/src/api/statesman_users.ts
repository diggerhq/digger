export async function getUserEmail(userId: string, orgId: string): Promise<string> {
    try {
        const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/users/${userId}`, {
            headers: {
                'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
                'X-Org-ID': orgId,
                'X-User-ID': userId,
                'X-Email': '',
            },
        });
        
        if (response.ok) {
            const userData = await response.json();
            return userData.email || '';
        }
        
        return '';
    } catch (error) {
        console.error('Error fetching user email:', error);
        return '';
    }
}

export async function syncUserToStatesman(userId: string, userEmail: string, orgId: string) {
    const response = await fetch(`${process.env.STATESMAN_BACKEND_URL}/internal/api/users`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${process.env.STATESMAN_BACKEND_WEBHOOK_SECRET}`,
            'X-Org-ID': orgId,
            'X-User-ID': userId,
            'X-Email': userEmail,
        },
        body: JSON.stringify({
            subject: userId,
            email: userEmail,
        })
    })

    if (response.status === 409) {
        console.log("User already exists in statesman")
        return response.json();
    }

    if (!response.ok) {
        throw new Error(`Failed to sync user: ${response.statusText}`);
    }

    return response.json();
}