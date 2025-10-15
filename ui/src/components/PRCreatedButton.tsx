'use client'

import { Button } from "@/components/ui/button"
import { GitPullRequest } from "lucide-react"

import { trackPRCreated } from "@/lib/analytics"

interface PRCreatedButtonProps {
  onClick: () => void
}

export function PRCreatedButton({ onClick }: PRCreatedButtonProps) {

  
  const handleClick = () => {

    onClick()
  }

  return (
    <Button onClick={handleClick}>
      <GitPullRequest className="mr-2 h-4 w-4" /> I've created the PR
    </Button>
  )
}