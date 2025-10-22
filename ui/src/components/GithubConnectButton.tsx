'use client'

import { Button } from "@/components/ui/button"
import { Github } from "lucide-react"


interface GithubConnectButtonProps {
  source: 'onboarding' | 'add_connection_dialog'
  onClick: () => void
}

export function GithubConnectButton({ source, onClick }: GithubConnectButtonProps) {

  const handleClick = () => {
    onClick()
  }

  return (
    <Button onClick={handleClick}>
      <Github className="mr-2 h-4 w-4" /> Connect with GitHub
    </Button>
  )
}