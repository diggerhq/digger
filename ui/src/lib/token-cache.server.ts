// Token verification cache - tokens are validated frequently in TFE proxy
// Cache valid tokens for 5 minutes to avoid repeated verification calls

interface TokenCacheEntry {
  userId: string;
  userEmail: string;
  orgId: string;
  expiresAt: number;
}

class TokenCache {
  private readonly cache = new Map<string, TokenCacheEntry>();
  private readonly TTL = 5 * 60 * 1000; // 5 minutes

  get(token: string): TokenCacheEntry | null {
    const entry = this.cache.get(token);
    if (!entry) return null;
    
    if (Date.now() > entry.expiresAt) {
      this.cache.delete(token);
      return null;
    }
    
    return entry;
  }

  set(token: string, userId: string, userEmail: string, orgId: string): void {
    this.cache.set(token, {
      userId,
      userEmail,
      orgId,
      expiresAt: Date.now() + this.TTL
    });
  }

  clear(token: string): void {
    this.cache.delete(token);
  }

  cleanExpired(): void {
    const now = Date.now();
    for (const [token, entry] of this.cache.entries()) {
      if (now > entry.expiresAt) {
        this.cache.delete(token);
      }
    }
  }

  getStats() {
    return {
      size: this.cache.size,
      entries: Array.from(this.cache.keys()).length
    };
  }
}

export const tokenCache = new TokenCache();

// Clean expired tokens every minute
setInterval(() => tokenCache.cleanExpired(), 60 * 1000);

