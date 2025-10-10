"use server";

import { updateOrgSettings } from "./api";

// helpers/slack.ts
// ──────────────────────────────────────────────────────────────────────────────
// CONFIG
// Prefer a bot token (xoxb-…), keep it server-side only.
const SLACK_TOKEN = process.env.SLACK_BOT_TOKEN as string;
if (!SLACK_TOKEN) {
  throw new Error("SLACK_BOT_TOKEN is not set");
}

const JSON_HEADERS = {
  "Content-Type": "application/json; charset=utf-8",
  "Authorization": `Bearer ${SLACK_TOKEN}`,
} as const;

// ──────────────────────────────────────────────────────────────────────────────
// CORE HTTP HELPERS (scope-aware)
async function slackFetch(
  url: string,
  init: RequestInit & { jsonBody?: unknown } = {}
) {
  const opts: RequestInit = {
    method: init.method || (init.jsonBody ? "POST" : "GET"),
    headers: {
      ...(init.headers || {}),
      Authorization: `Bearer ${SLACK_TOKEN}`,
      ...(init.jsonBody ? { "Content-Type": "application/json; charset=utf-8" } : {}),
    },
    body: init.jsonBody ? JSON.stringify(init.jsonBody) : init.body,
  };

  const res = await fetch(url, opts);
  let data: any;
  try {
    data = await res.json();
  } catch {
    // Slack should always return JSON, but just in case:
    const text = await res.text().catch(() => "");
    throw new Error(`slack_non_json_response: ${res.status} ${text}`);
  }

  if (!data.ok) {
    // Enrich error with useful context
    const err = new Error(data.error || "slack_error") as Error & {
      slack?: any;
    };
    err.slack = {
      url,
      httpStatus: res.status,
      error: data.error,
      needed: data.needed,
      provided: data.provided,
      response: data,
    };
    throw err;
  }

  return data;
}

// Convenience wrappers
async function slackGet(url: string) {
  return slackFetch(url, { method: "GET" });
}
async function slackPost(url: string, body: unknown) {
  return slackFetch(url, { jsonBody: body });
}

// ──────────────────────────────────────────────────────────────────────────────
/** One-time self-check. Useful during cold starts / first invocation.
 * Confirms token identity and team; logs to server console once.
 */
let _authChecked = false;
async function ensureAuthOnce() {
  if (_authChecked) return;
  try {
    const who = await slackGet("https://slack.com/api/auth.test");
    console.log("[slack] auth.test ok", {
      team: who.team,
      team_id: who.team_id,
      user: who.user,
      url: who.url,
      bot_id: who.bot_id,
    });
    _authChecked = true;
  } catch (e: any) {
    console.error("[slack] auth.test failed", e?.slack || e);
    throw new Error("Slack auth failed. Check SLACK_BOT_TOKEN.");
  }
}

// ──────────────────────────────────────────────────────────────────────────────
// UTILITIES
function sanitizeChannelName(name: string) {
  // Slack requires lowercase, no spaces, limited chars. Replace spaces with dashes.
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9-_]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80); // Slack name limit
}

function chunk<T>(arr: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < arr.length; i += size) out.push(arr.slice(i, i + size));
  return out;
}

// ──────────────────────────────────────────────────────────────────────────────
// SLACK FEATURE HELPERS

// 1) Create a (private) channel
export async function createChannel(name: string, isPrivate = true) {
  await ensureAuthOnce();
  const sanitized = sanitizeChannelName(name);
  try {
    const data = await slackPost("https://slack.com/api/conversations.create", {
      name: sanitized,
      is_private: isPrivate,
    });
    return (data as any).channel.id as string; // e.g., "C0123456789"
  } catch (e: any) {
    // Most common scope misses here:
    // - private channel: needs groups:write
    // - public channel: needs channels:manage
    if (e?.slack?.error === "missing_scope") {
      console.error("[slack] missing_scope (create):", {
        needed: e.slack.needed,
        provided: e.slack.provided,
      });
      throw new Error(
        `Slack missing_scope for conversations.create. Needed: ${e.slack.needed}. ` +
          `Add the scope in your app (OAuth & Permissions) and reinstall.`
      );
    }
    throw e;
  }
}

// 2) Resolve internal emails -> user IDs (or cache via users.list)
export async function userIdByEmail(email: string) {
  await ensureAuthOnce();
  try {
    const data = await slackGet(
      `https://slack.com/api/users.lookupByEmail?email=${encodeURIComponent(email)}`
    );
    return (data as any).user.id as string; // e.g., "U0123456789"
  } catch (e: any) {
    if (e?.slack?.error === "users_not_found" || e?.slack?.error === "user_not_found") {
      throw new Error(`Slack user not found for email: ${email}`);
    }
    if (e?.slack?.error === "missing_scope") {
      // Typically needs users:read.email (and users:read)
      throw new Error(
        `Slack missing_scope for users.lookupByEmail (likely users:read.email). Needed: ${e.slack.needed}`
      );
    }
    throw e;
  }
}

// 3) Invite internal members (CSV of IDs). Batches for large lists.
export async function inviteInternal(channelId: string, userIds: string[]) {
  await ensureAuthOnce();
  if (!userIds.length) return;

  // Slack allows up to 1000 IDs but we’ll be conservative to simplify retries.
  for (const batch of chunk(userIds, 500)) {
    try {
      await slackPost("https://slack.com/api/conversations.invite", {
        channel: channelId,
        users: batch.join(","), // CSV
      });
    } catch (e: any) {
      // Common: already_in_channel / already_in_group, user_not_found, not_in_channel (bot must be in channel)
      if (e?.slack?.error === "already_in_channel" || e?.slack?.error === "already_in_group") {
        continue; // harmless
      }
      if (e?.slack?.error === "missing_scope") {
        // public: channels:manage; private: groups:write
        throw new Error(
          `Slack missing_scope for conversations.invite. Needed: ${e.slack.needed}. ` +
            `If channel is private, ensure groups:write; if public, ensure channels:manage.`
        );
      }
      throw e;
    }
  }
}

// 4) Share with external org via Slack Connect (emails OR user_ids in other org)
export async function inviteExternalViaConnect(channelId: string, externalEmails: string[]) {
  await ensureAuthOnce();
  if (!externalEmails?.length) return;

  try {
    await slackPost("https://slack.com/api/conversations.inviteShared", {
      channel: channelId,
      emails: externalEmails, // array of strings
    });
  } catch (e: any) {
    if (e?.slack?.error === "missing_scope") {
      // Needs conversations.connect:write
      throw new Error(
        `Slack missing_scope for conversations.inviteShared. Needed: ${e.slack.needed} (add conversations.connect:write) and reinstall.`
      );
    }
    // Other policy-related errors could include 'not_allowed' or admin approval requirements.
    throw e;
  }
}

// (Optional) If your workspace requires approval-before-sending or post-accept approval:
export async function listPendingConnectInvites() {
  await ensureAuthOnce();
  try {
    const data = await slackGet("https://slack.com/api/conversations.listConnectInvites?limit=200");
    return (data as any).invites as any[];
  } catch (e: any) {
    if (e?.slack?.error === "missing_scope") {
      // Needs conversations.connect:read
      throw new Error(
        `Slack missing_scope for conversations.listConnectInvites. Needed: ${e.slack.needed} (add conversations.connect:read) and reinstall.`
      );
    }
    throw e;
  }
}

// ──────────────────────────────────────────────────────────────────────────────
// HIGH-LEVEL FLOW

/** Creates a private channel, invites internal team (emails → IDs), then invites external emails via Slack Connect.
 * Returns { success, channelId, invites, error }
 */
// Using global fetch from Next.js/Node 18+. No need to import 'node-fetch'.

export async function createSlackChannel(email: string, channelName: string) {
  console.log("Creating Slack channel:", { channelName, externalEmail: email });
  try {
    // 1) Create channel (private)
    const channelId = await createChannel(channelName, true);
    console.log("Channel created:", channelId);

    // 2) Internal team — put real internal emails here
    // NOTE: your example had "mo_digger.dev" which is not a valid email.
    const internalEmails = process.env.SLACK_TEAM_INTERNAL_EMAILS_TO_INVITE?.split(";") || [];
    const internalIds = await Promise.all(internalEmails.map(userIdByEmail).map(p => p.catch(err => err)));
    const resolvedIds = internalIds.filter((x): x is string => typeof x === "string");
    const failedLookups = internalIds.filter((x) => x instanceof Error) as Error[];

    if (resolvedIds.length) {
      await inviteInternal(channelId, resolvedIds);
    }
    if (failedLookups.length) {
      console.warn("[slack] some internal lookups failed:", failedLookups.map(f => f.message));
    }

    // 3) Invite external via Slack Connect
    await inviteExternalViaConnect(channelId, [email]);

    // 4) (Optional) Surface pending invites
    let invites: any[] | null = null;
    try {
      invites = await listPendingConnectInvites();
    } catch (e) {
      // If connect:read is missing, we still succeeded creating & inviting.
      console.warn("[slack] listPendingConnectInvites skipped:", (e as any)?.message || e);
    }

    return { success: true, channelId, invites, error: null };
  } catch (error: any) {
    // Unwrap enriched slack error if present
    const errPayload = error?.slack || { message: error?.message || String(error) };
    console.error("Error creating Slack channel:", errPayload);
    return {
      success: false,
      error:
        error?.message ||
        error?.slack?.error ||
        "Unknown error occurred",
      channelId: null,
      invites: null,
    };
  }
}
