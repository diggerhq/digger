import OnboardingSteps from '@/components/OnboardingSteps'
import { Button } from '@/components/ui/button'
import { createFileRoute, Link, useRouter } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'
import { useState } from 'react'


interface RepoInfo {
    name: string
    owner: string
  }

export const Route = createFileRoute(
  '/_authenticated/_dashboard/dashboard/onboarding',
)({
  component: RouteComponent,
  loader: async ({ context }) => {
    const { user, organisationId, publicServerConfig } = context
    const publicHostname = publicServerConfig?.PUBLIC_HOSTNAME || ''
    const githubAppUrl = publicServerConfig?.ORCHESTRATOR_GITHUB_APP_URL || ''
    return { user, organisationId, publicHostname, githubAppUrl  }
  },
})

function RouteComponent() {
    const { user, organisationId, publicHostname, githubAppUrl } = Route.useLoaderData()
    const [repoInfo, setRepoInfo] = useState<RepoInfo | null>(null)
    const router = useRouter()
    const handleOnboardingComplete = () => {
        router.navigate({ to: "/dashboard/units" })
      }

  return (
    <div className="container mx-auto p-4">
    <div className="mb-6">
      <Button variant="ghost" asChild>
        <Link to="/dashboard/repos">
          <ArrowLeft className="mr-2 h-4 w-4" /> Back to Dashboard
        </Link>
      </Button>
    </div>
    {repoInfo ? (
      <OnboardingSteps
        repoName={repoInfo.name}
        repoOwner={repoInfo.owner}
        userId={user?.id || ''}
        email={user?.email || ''}
        organisationId={organisationId || ''}
        publicHostname={publicHostname}
        githubAppUrl={githubAppUrl}
        onComplete={handleOnboardingComplete}
      />
    ) : (
      <OnboardingSteps
        userId={user?.id || ''}
        email={user?.email || ''}
        organisationId={organisationId || ''}
        publicHostname={publicHostname}
        githubAppUrl={githubAppUrl}
        onComplete={handleOnboardingComplete}
      />
    )}
  </div>    
  )
}
