// Helper to generate request IDs for tracing
function generateRequestId(): string {
    return `ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

export const getTokens = async (organizationId: string, userId: string, page: number = 1, pageSize: number = 20) => {
    const query = new URLSearchParams({ 
        org_id: organizationId, 
        user_id: userId,
        page: String(page),
        page_size: String(pageSize),
    });
    const url = `${process.env.TOKENS_SERVICE_BACKEND_URL}/api/v1/tokens?${query.toString()}`;
    const response = await fetch(url, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
            'Cache-Control': 'no-cache',
            'Pragma': 'no-cache',
            'X-Request-ID': generateRequestId(),
        },
        // Disable browser caching for token requests
        cache: 'no-store',
    })
    if (!response.ok) {
        throw new Error(`Failed to get tokens: ${response.statusText}`);
    }
    return response.json();
}

export const createToken = async (organizationId: string, userId: string, name: string, expiresAt: string | null ) => {
    const response = await fetch(`${process.env.TOKENS_SERVICE_BACKEND_URL}/api/v1/tokens`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-Request-ID': generateRequestId(),
        },
        body: JSON.stringify({
            org_id: organizationId,
            user_id: userId,
            name: name,
            expires_in: expiresAt,
        }),
    })
    if (!response.ok) {
        throw new Error(`Failed to create token: ${response.statusText}`);
    }
    return response.json();
}

export const verifyToken = async (token: string) => {
    const response = await fetch(`${process.env.TOKENS_SERVICE_BACKEND_URL}/api/v1/tokens/verify`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-Request-ID': generateRequestId(),
        },
        body: JSON.stringify({
            token: token,
        }),
    })
    if (!response.ok) {
        throw new Error(`Failed to verify token: ${response.statusText}`);
    }
    return response.json();
}

export const deleteToken = async (organizationId: string, userId: string, tokenId: string) => {
    const response = await fetch(`${process.env.TOKENS_SERVICE_BACKEND_URL}/api/v1/tokens/${tokenId}`, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json',
            'X-Request-ID': generateRequestId(),
        },
        body: JSON.stringify({
            org_id: organizationId,
            user_id: userId,
        }),
    })
    if (!response.ok) {
        throw new Error(`Failed to delete token: ${response.statusText}`);
    }
    return response.json();
}
