
export const getTokens = async (organizationId: string, userId: string) => {
    const query = new URLSearchParams({ org_id: organizationId, user_id: userId });
    const url = `${process.env.TOKENS_SERVICE_BACKEND_URL}/api/v1/tokens?${query.toString()}`;
    const response = await fetch(url, {
        method: 'GET',
        headers: {
            'Content-Type': 'application/json',
        },
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
