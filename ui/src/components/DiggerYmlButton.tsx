'use client'

import { Button } from "@/components/ui/button"
import { FileText } from "lucide-react"

interface DiggerYmlButtonProps {
  onClick: () => void
}

export function DiggerYmlButton({ onClick }: DiggerYmlButtonProps) { 
  const handleClick = () => {
    onClick()
  }

  return (
    <Button onClick={handleClick}>
      <FileText className="mr-2 h-4 w-4" /> I've added the digger.yml file
    </Button>
  )
}