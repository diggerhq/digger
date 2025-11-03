// Server-side in-memory cache for WorkOS organization details ONLY
// 
// ⚠️ IMPORTANT: This cache is ONLY for WorkOS organization metadata (name, id)
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

class OrgCache {
  private readonly orgCache = new Map<string, CacheEntry<Organization>>();
  
  // Cache organization details for 5 minutes
  // Org names/details rarely change, so this is safe
  private readonly ORG_TTL = 5 * 60 * 1000;

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

  // Clear expired entries periodically
  cleanExpired(): void {
    const now = Date.now();
    for (const [key, entry] of this.orgCache.entries()) {
      if (now > entry.expiresAt) {
        this.orgCache.delete(key);
      }
    }
  }

  // Get cache stats for monitoring
  getStats() {
    return {
      orgCacheSize: this.orgCache.size,
    };
  }
}

// Single instance per server process
export const serverCache = new OrgCache();

// Clean up expired entries every minute
setInterval(() => serverCache.cleanExpired(), 60 * 1000);

