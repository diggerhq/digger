'use client'

import { Button } from "@/components/ui/button"
import { Github } from "lucide-react"


interface GithubConnectButtonProps {
  source: 'onboarding' | 'add_connection_dialog'
  onClick: () => void
  user?: { id?: string; email?: string }
  organizationId?: string
}

import { trackConnectWithGithub } from '@/lib/analytics'

export function GithubConnectButton({ source, onClick, user, organizationId }: GithubConnectButtonProps) {

  const handleClick = () => {
    trackConnectWithGithub(source, user, organizationId)
    onClick()
  }

  return (
    <Button onClick={handleClick}>
      <Github className="mr-2 h-4 w-4" /> Connect with GitHub
    </Button>
  )
}