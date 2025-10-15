function redirectWithFallback(redirectUri: string, headers?: Headers) {
    const newHeaders = headers ? new Headers(headers) : new Headers();
    newHeaders.set('Location', redirectUri);
  
    return new Response(null, { status: 307, headers: newHeaders });
  }
  
  function errorResponseWithFallback(errorBody: { error: { message: string; description: string } }) {
    return new Response(JSON.stringify(errorBody), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    });
  }

export { redirectWithFallback, errorResponseWithFallback };