// No client-side Stripe SDK needed for redirect with v8; use session URL

export default function SubscribeButton() {
  const handleClick = async () => {
    const res = await fetch("/api/stripe/checkout-session", {
      method: "POST",
    });
    const { url } :any = await res.json();
    if (typeof url === "string" && url.length > 0) {
      window.location.assign(url);
    }
  };

  return (
    <button
      onClick={handleClick}
      className="inline-flex items-center justify-center px-4 py-2 font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors ml-auto"
    >
      Upgrade to Pro
    </button>
  );
}
