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
})

function RouteComponent() {
    const [repoInfo, setRepoInfo] = useState<RepoInfo | null>(null)
    const router = useRouter()
    const handleOnboardingComplete = () => {
        router.navigate({ to: "/dashboard/repos" })
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
        onComplete={handleOnboardingComplete}
      />
    ) : (
      <OnboardingSteps onComplete={handleOnboardingComplete} />
    )}
  </div>    
  )
}
