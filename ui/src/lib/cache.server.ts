// Aggressive server-side caching for WorkOS data
// 
// ⚠️ IMPORTANT: This cache is ONLY for WorkOS metadata
// DO NOT cache core application data like:
// - Units, unit versions, unit status
// - Projects, project status
// - Repos, jobs
// - Any data that users actively modify
//
// This module is evaluated once per server process and cached by Node's module system.

import { Organization } from '@workos-inc/node';

interface CacheEntry<T> {
  data: T;
  expiresAt: number;
}

interface UserMembership {
  userId: string;
  memberships: any[]; // Store full membership objects from WorkOS
}

class AggressiveWorkOSCache {
  // Organization cache - 30 minutes (org names rarely change)
  private readonly orgCache = new Map<string, CacheEntry<Organization>>();
  private readonly ORG_TTL = 30 * 60 * 1000; // 30 minutes

  // User memberships cache - 15 minutes (users don't switch orgs often)
  private readonly membershipCache = new Map<string, CacheEntry<UserMembership>>();
  private readonly MEMBERSHIP_TTL = 15 * 60 * 1000; // 15 minutes

  // Widget tokens cache - 5 minutes (tokens expire quickly)
  private readonly widgetTokenCache = new Map<string, CacheEntry<string>>();
  private readonly WIDGET_TOKEN_TTL = 5 * 60 * 1000; // 5 minutes

  // ============================================================================
  // ORGANIZATION CACHE
  // ============================================================================

  getOrg(orgId: string): Organization | null {
    const entry = this.orgCache.get(orgId);
    if (!entry) return null;
    
    if (Date.now() > entry.expiresAt) {
      this.orgCache.delete(orgId);
      return null;
    }
    
    return entry.data;
  }

  setOrg(orgId: string, org: Organization): void {
    this.orgCache.set(orgId, {
      data: org,
      expiresAt: Date.now() + this.ORG_TTL,
    });
  }

  clearOrg(orgId: string): void {
    this.orgCache.delete(orgId);
  }

  // ============================================================================
  // USER MEMBERSHIPS CACHE
  // ============================================================================

  getUserMemberships(userId: string): any[] | null {
    const entry = this.membershipCache.get(userId);
    if (!entry) return null;
    
    if (Date.now() > entry.expiresAt) {
      this.membershipCache.delete(userId);
      return null;
    }
    
    return entry.data.memberships;
  }

  setUserMemberships(userId: string, memberships: any[]): void {
    this.membershipCache.set(userId, {
      data: { userId, memberships },
      expiresAt: Date.now() + this.MEMBERSHIP_TTL,
    });
  }

  clearUserMemberships(userId: string): void {
    this.membershipCache.delete(userId);
  }

  // ============================================================================
  // WIDGET TOKEN CACHE
  // ============================================================================

  getWidgetToken(userId: string, orgId: string): string | null {
    const key = `${userId}:${orgId}`;
    const entry = this.widgetTokenCache.get(key);
    if (!entry) return null;
    
    if (Date.now() > entry.expiresAt) {
      this.widgetTokenCache.delete(key);
      return null;
    }
    
    return entry.data;
  }

  setWidgetToken(userId: string, orgId: string, token: string): void {
    const key = `${userId}:${orgId}`;
    this.widgetTokenCache.set(key, {
      data: token,
      expiresAt: Date.now() + this.WIDGET_TOKEN_TTL,
    });
  }

  clearWidgetToken(userId: string, orgId: string): void {
    const key = `${userId}:${orgId}`;
    this.widgetTokenCache.delete(key);
  }

  // ============================================================================
  // CACHE MANAGEMENT
  // ============================================================================

  cleanExpired(): void {
    const now = Date.now();
    
    // Clean org cache
    for (const [key, entry] of this.orgCache.entries()) {
      if (now > entry.expiresAt) {
        this.orgCache.delete(key);
      }
    }
    
    // Clean membership cache
    for (const [key, entry] of this.membershipCache.entries()) {
      if (now > entry.expiresAt) {
        this.membershipCache.delete(key);
      }
    }
    
    // Clean widget token cache
    for (const [key, entry] of this.widgetTokenCache.entries()) {
      if (now > entry.expiresAt) {
        this.widgetTokenCache.delete(key);
      }
    }
  }

  getStats() {
    return {
      orgCacheSize: this.orgCache.size,
      membershipCacheSize: this.membershipCache.size,
      widgetTokenCacheSize: this.widgetTokenCache.size,
    };
  }

  // Invalidate all caches for a user (when they switch orgs, etc.)
  invalidateUser(userId: string): void {
    this.clearUserMemberships(userId);
    // Clear all widget tokens for this user
    for (const key of this.widgetTokenCache.keys()) {
      if (key.startsWith(`${userId}:`)) {
        this.widgetTokenCache.delete(key);
      }
    }
  }

  // Invalidate all caches for an org (when org details change)
  invalidateOrg(orgId: string): void {
    this.clearOrg(orgId);
    // Clear all widget tokens for this org
    for (const key of this.widgetTokenCache.keys()) {
      if (key.endsWith(`:${orgId}`)) {
        this.widgetTokenCache.delete(key);
      }
    }
  }
}

// Single instance per server process
export const serverCache = new AggressiveWorkOSCache();

// Clean up expired entries every minute
setInterval(() => serverCache.cleanExpired(), 60 * 1000);

