import { Button } from "@/components/ui/button"

export default function GitHubConnection({ githubAppUrl }: { githubAppUrl: string }) {
  return (
    <div className="space-y-4">
      <p>To connect your GitHub repository, you need to install our GitHub App.</p>
      <Button asChild>
        <a href={githubAppUrl} target="_blank" rel="noopener noreferrer">
          Install GitHub App
        </a>
      </Button>
    </div>
  )
}

