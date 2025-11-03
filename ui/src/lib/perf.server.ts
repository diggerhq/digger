// Server-side performance logging utilities

export async function timeAsync<T>(
  label: string,
  fn: () => Promise<T>,
  warnThreshold: number = 500
): Promise<T> {
  const start = Date.now();
  try {
    const result = await fn();
    const duration = Date.now() - start;
    
    if (duration > warnThreshold) {
      console.log(`⚠️  SLOW: ${label} took ${duration}ms`);
    } else if (duration > 200) {
      console.log(`⏱️  ${label} took ${duration}ms`);
    } else {
      console.log(`✅ ${label} took ${duration}ms`);
    }
    
    return result;
  } catch (error) {
    const duration = Date.now() - start;
    console.error(`❌ ${label} failed after ${duration}ms:`, error);
    throw error;
  }
}

export function createTimer() {
  const start = Date.now();
  return {
    elapsed: () => Date.now() - start,
    log: (label: string) => {
      const duration = Date.now() - start;
      console.log(`⏱️  ${label}: ${duration}ms`);
    }
  };
}

