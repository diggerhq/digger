import { Button } from "@/components/ui/button"

export default function GitHubConnection() {
  return (
    <div className="space-y-4">
      <p>To connect your GitHub repository, you need to install our GitHub App.</p>
      <Button asChild>
        <a href="https://github.com/apps/digger-pro" target="_blank" rel="noopener noreferrer">
          Install GitHub App
        </a>
      </Button>
    </div>
  )
}

