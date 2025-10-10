"use client"

import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Home, GitBranch, FolderOpen, Settings, Shield, Network, LogOut, ChevronLeft, Menu } from "lucide-react"
import { cn } from "@/lib/utils"

interface SidebarProps {
  className?: string
}

export function Sidebar({ className }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(false)

  const menuItems = [
    { icon: Home, label: "Home", href: "/" },
    { icon: GitBranch, label: "Repos", href: "/repos" },
    { icon: FolderOpen, label: "Projects", href: "/projects", active: true },
    { icon: Network, label: "Connections", href: "/connections" },
    { icon: Shield, label: "Policies", href: "/policies" },
    { icon: Settings, label: "Settings", href: "/settings" },
  ]

  return (
    <div
      className={cn(
        "flex flex-col h-screen bg-gray-50 border-r border-gray-200 transition-all duration-300",
        collapsed ? "w-16" : "w-64",
        className,
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between p-4 border-b border-gray-200">
        {!collapsed && <h1 className="text-xl font-semibold text-gray-900">Drift</h1>}
        <Button variant="ghost" size="sm" onClick={() => setCollapsed(!collapsed)} className="p-2">
          {collapsed ? <Menu className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
        </Button>
      </div>

      {/* Menu Label */}
      {!collapsed && (
        <div className="px-4 py-2">
          <span className="text-xs font-medium text-gray-500 uppercase tracking-wider">Menu</span>
        </div>
      )}

      {/* Navigation */}
      <nav className="flex-1 px-2 py-2 space-y-1">
        {menuItems.map((item) => (
          <Button
            key={item.label}
            variant={item.active ? "secondary" : "ghost"}
            className={cn(
              "w-full justify-start text-gray-700 hover:text-gray-900 hover:bg-gray-100",
              item.active && "bg-gray-100 text-gray-900",
              collapsed && "justify-center px-2",
            )}
          >
            <item.icon className={cn("w-5 h-5", !collapsed && "mr-3")} />
            {!collapsed && <span>{item.label}</span>}
          </Button>
        ))}
      </nav>

      {/* Logout */}
      <div className="p-2 border-t border-gray-200">
        <Button
          variant="ghost"
          className={cn(
            "w-full justify-start text-gray-700 hover:text-gray-900 hover:bg-gray-100",
            collapsed && "justify-center px-2",
          )}
        >
          <LogOut className={cn("w-5 h-5", !collapsed && "mr-3")} />
          {!collapsed && <span>Logout</span>}
        </Button>
      </div>
    </div>
  )
}
