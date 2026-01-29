import { createServerFn } from "@tanstack/react-start";
import { createToken, getTokens } from "./tokens";
import { verifyToken } from "./tokens";
import { deleteToken } from "./tokens";

export const getTokensFn = createServerFn({method: 'GET'})
    .inputValidator((data: {organizationId: string, userId: string, page?: number, pageSize?: number}) => data)
    .handler(async ({data: {organizationId, userId, page = 1, pageSize = 20}}) => {
        return getTokens(organizationId, userId, page, pageSize);
})

export const createTokenFn = createServerFn({method: 'POST'})
    .inputValidator((data: {organizationId: string, userId: string, name: string, expiresAt: string | null}) => data)
    .handler(async ({data: {organizationId, userId, name, expiresAt}}) => {
        return createToken(organizationId, userId, name, expiresAt);
})

export const verifyTokenFn = createServerFn({method: 'POST'})
    .inputValidator((data: { token: string}) => data)
    .handler(async ({data: { token}}) => {
        return verifyToken( token);
})

export const deleteTokenFn = createServerFn({method: 'POST'})
    .inputValidator((data: {organizationId: string, userId: string, tokenId: string}) => data)
    .handler(async ({data: {organizationId, userId, tokenId}}) => {
        return deleteToken(organizationId, userId, tokenId);
})
