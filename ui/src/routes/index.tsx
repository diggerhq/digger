import { createFileRoute } from '@tanstack/react-router';
import { getSignInUrl } from '../authkit/serverFunctions';
import SignInButton from '../components/sign-in-button';
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from '../components/ui/collapsible';

export const Route = createFileRoute('/')({
  component: Home,
  loader: async ({ context }) => {
    const { user } = context;
    const url = await getSignInUrl();
    return { user, url };
  },
});

function Home() {
  const { user, url } = Route.useLoaderData();

  return (
    <div className="relative min-h-screen overflow-hidden bg-gradient-to-b from-black via-slate-900 to-slate-950 text-white">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -top-40 left-1/2 h-96 w-[60rem] -translate-x-1/2 rounded-full bg-[radial-gradient(ellipse_at_center,rgba(56,189,248,0.15),transparent_60%)] blur-2xl" />
        <div className="absolute bottom-0 right-0 h-72 w-72 rounded-full bg-[radial-gradient(circle_at_center,rgba(139,92,246,0.2),transparent_60%)] blur-xl" />
      </div>

      <div className="relative mx-auto flex min-h-screen max-w-7xl items-center px-6">
        <div className="max-w-2xl">
          {user ? (
            <>
              <span className="inline-flex items-center rounded-full border border-white/10 bg-white/5 px-3 py-1 text-sm text-white/70 backdrop-blur">Welcome back</span>
              <h1 className="mt-6 text-5xl font-extrabold tracking-tight sm:text-6xl">OpenTACO
                <span className="block bg-gradient-to-r from-cyan-300 via-sky-400 to-violet-400 bg-clip-text text-transparent text-5xl">The future of IAC is open</span>
              </h1>
              <p className="mt-4 text-lg text-white/70">You're signed in. Head over to your dashboard to continue.</p>
              <div className="mt-8 flex items-center gap-4">
                <a href="/dashboard/repos" className="inline-flex items-center rounded-md bg-white px-5 py-3 font-medium text-black shadow-lg shadow-white/10 transition hover:bg-white/90">Go to dashboard</a>
                <a href="/logout" className="inline-flex items-center rounded-md bg-transparent border border-white px-5 py-3 font-medium text-white shadow-lg shadow-white/10 transition hover:bg-white/10">Logout</a>
              </div>
            </>
          ) : (
            <>
              <span className="inline-flex items-center rounded-full border border-white/10 bg-white/5 px-3 py-1 text-sm text-white/70 backdrop-blur">Open. Composable. Powerful.</span>
              <h1 className="mt-6 text-5xl font-extrabold tracking-tight sm:text-6xl">OpenTACO
              <span className="block bg-gradient-to-r from-cyan-300 via-sky-400 to-violet-400 bg-clip-text text-transparent text-5xl">The future of IAC is open</span>
              </h1>
              <p className="mt-4 text-lg text-white/70">Build, manage, and collaborate on Infrastructure-as-Code with an open, interoperable toolkit.</p>
              <div className="mt-8 flex items-center gap-4">
                <SignInButton user={user} url={url} large />
              </div>
            </>
          )}

          {/* Redirect note for ui.digger.dev visitors */}
          <div className="mt-8">
            <Collapsible defaultOpen={false}>
              <CollapsibleTrigger className="w-full text-left rounded-md border border-white/10 bg-white/5 px-4 py-3 text-white transition hover:bg-white/10">
                redirected from ui.digger.dev?
              </CollapsibleTrigger>
              <CollapsibleContent className="mt-3 rounded-md border border-white/10 bg-black/30 p-4 text-white/80">
                <p>
                  The project formerly known as <span className="font-semibold">diggerhq/digger</span> has been
                  rebranded to <span className="font-semibold">OpenTaco</span>. It continues to provide the same
                  functionality and compatibility, now with more features including
                  <span className="font-semibold"> state management</span>, <span className="font-semibold">remote runs</span>,
                  and <span className="font-semibold">RBAC</span>. If you intended to sign in to ui.digger.dev your account will work the same way when you sign in here.
                </p>
              </CollapsibleContent>
            </Collapsible>
          </div>
        </div>
      </div>
    </div>
  );
}
 