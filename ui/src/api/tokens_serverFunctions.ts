import { createServerFn } from "@tanstack/react-start";
import { createToken, getTokens } from "./tokens";
import { verifyToken } from "./tokens";
import { deleteToken } from "./tokens";
import { requireAuth } from "./helpers";

export const getTokensFn = createServerFn({method: 'GET'})
    .handler(async () => {
        const auth = await requireAuth();
        return getTokens(auth.organizationId, auth.userId);
})

export const createTokenFn = createServerFn({method: 'POST'})
    .inputValidator((data: {name: string, expiresAt: string | null}) => data)
    .handler(async ({data: {name, expiresAt}}) => {
        const auth = await requireAuth();
        return createToken(auth.organizationId, auth.userId, name, expiresAt);
})

export const verifyTokenFn = createServerFn({method: 'POST'})
    .inputValidator((data: { token: string}) => data)
    .handler(async ({data: { token}}) => {
        return verifyToken( token);
})

export const deleteTokenFn = createServerFn({method: 'POST'})
    .inputValidator((data: {tokenId: string}) => data)
    .handler(async ({data: {tokenId}}) => {
        const auth = await requireAuth();
        return deleteToken(auth.organizationId, auth.userId, tokenId);
})
