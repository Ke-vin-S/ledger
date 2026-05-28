"use client";

import { useEffect } from "react";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import React from "react";
import {
  LayoutDashboard,
  Users,
  CreditCard,
  Bell,
  Settings,
  Menu,
  X,
  LogOut,
  Plus,
  ChevronRight,
  Sun,
  Moon,
} from "lucide-react";
import { useMe, useLogout } from "@/hooks/useAuth";
import { useTeams } from "@/hooks/useTeam";
import { useUIStore } from "@/store/ui";
import { useNotifications } from "@/hooks/useNotifications";
import { Avatar } from "@/components/shared/Avatar";
import { AddExpenseSheet } from "@/components/expense/AddExpenseSheet";
import { cn } from "@/lib/utils";

const navItems: { href: string; label: string; icon: React.ElementType }[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/loans", label: "Loans", icon: CreditCard },
  { href: "/notifications", label: "Notifications", icon: Bell },
  { href: "/settings", label: "Settings", icon: Settings },
];

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const { data: me, isLoading, isError } = useMe();
  const { data: teams } = useTeams();
  const { sidebarOpen, toggleSidebar, theme, setTheme } = useUIStore();
  const { mutate: logout } = useLogout();
  const { data: notifications } = useNotifications(true);

  useEffect(() => {
    if (!isLoading && isError) {
      router.push("/login");
    }
  }, [isLoading, isError, router]);

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 rounded-full bg-[hsl(var(--primary))] opacity-20 animate-pulse" />
          <p className="text-[hsl(var(--muted-foreground))] text-sm">Loading…</p>
        </div>
      </div>
    );
  }

  if (!me) return null;

  const unreadCount = notifications?.items.length ?? 0;

  function toggleTheme() {
    setTheme(theme === "dark" ? "light" : "dark");
  }

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside
        className={cn(
          "flex flex-col bg-[hsl(var(--card))] border-r transition-all duration-200 flex-shrink-0 relative",
          sidebarOpen ? "w-60" : "w-14",
        )}
      >
        {/* Header */}
        <div className="flex items-center h-14 px-3 border-b gap-2 flex-shrink-0">
          <button
            onClick={toggleSidebar}
            className="p-1.5 rounded-lg hover:bg-[hsl(var(--muted))] transition-colors flex-shrink-0"
            aria-label="Toggle sidebar"
          >
            {sidebarOpen ? <X className="h-4 w-4" /> : <Menu className="h-4 w-4" />}
          </button>
          {sidebarOpen && (
            <span className="font-semibold text-sm truncate tracking-tight">SplitLedger</span>
          )}
          {sidebarOpen && (
            <button
              onClick={toggleTheme}
              className="ml-auto p-1.5 rounded-lg hover:bg-[hsl(var(--muted))] transition-colors"
              aria-label="Toggle theme"
            >
              {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </button>
          )}
        </div>

        {/* FAB — Add Expense */}
        <div className="px-2 pt-3 pb-1 flex-shrink-0">
          <AddExpenseSheet>
            <button
              className={cn(
                "flex items-center gap-2 w-full rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                "bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] hover:opacity-90",
                !sidebarOpen && "justify-center px-2",
              )}
            >
              <Plus className="h-4 w-4 flex-shrink-0" />
              {sidebarOpen && <span>Add Expense</span>}
            </button>
          </AddExpenseSheet>
        </div>

        {/* Nav items */}
        <nav className="flex-1 py-2 space-y-0.5 px-2 overflow-y-auto">
          {navItems.map(({ href, label, icon: Icon }) => {
            const isActive = pathname === href || pathname.startsWith(href + "/");
            return (
              <Link
                key={href}
                href={href as never}
                className={cn(
                  "flex items-center gap-3 px-2 py-2 rounded-lg text-sm transition-colors group relative",
                  isActive
                    ? "bg-[hsl(var(--accent))] text-[hsl(var(--accent-foreground))] font-medium"
                    : "hover:bg-[hsl(var(--muted))] text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]",
                  !sidebarOpen && "justify-center",
                )}
              >
                <div className="relative flex-shrink-0">
                  <Icon className="h-4 w-4" />
                  {href === "/notifications" && unreadCount > 0 && (
                    <span className="absolute -top-1 -right-1 h-3.5 w-3.5 rounded-full bg-[hsl(var(--destructive))] text-[hsl(var(--destructive-foreground))] text-[8px] flex items-center justify-center font-bold">
                      {unreadCount > 9 ? "9+" : unreadCount}
                    </span>
                  )}
                </div>
                {sidebarOpen && <span>{label}</span>}
              </Link>
            );
          })}

          {/* Teams section */}
          {sidebarOpen && (
            <div className="pt-3 pb-1">
              <Link
                href="/teams"
                className="flex items-center justify-between px-2 py-1.5 rounded-lg hover:bg-[hsl(var(--muted))] transition-colors group"
              >
                <div className="flex items-center gap-2">
                  <Users className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
                  <span className="text-xs font-semibold text-[hsl(var(--muted-foreground))] uppercase tracking-wider">
                    Teams
                  </span>
                </div>
                <ChevronRight className="h-3 w-3 text-[hsl(var(--muted-foreground))] opacity-0 group-hover:opacity-100 transition-opacity" />
              </Link>
              <div className="space-y-0.5 mt-0.5">
                {teams?.slice(0, 6).map((team) => {
                  const isActive = pathname.startsWith(`/teams/${team.id}`);
                  return (
                    <Link
                      key={team.id}
                      href={`/teams/${team.id}`}
                      className={cn(
                        "flex items-center gap-2 pl-6 pr-2 py-1.5 rounded-lg text-sm transition-colors truncate",
                        isActive
                          ? "bg-[hsl(var(--accent))] text-[hsl(var(--accent-foreground))] font-medium"
                          : "text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--muted))] hover:text-[hsl(var(--foreground))]",
                      )}
                    >
                      <span className="h-1.5 w-1.5 rounded-full bg-[hsl(var(--primary))] flex-shrink-0" />
                      <span className="truncate">{team.name}</span>
                    </Link>
                  );
                })}
                {(teams?.length ?? 0) === 0 && (
                  <Link
                    href="/teams"
                    className="flex items-center gap-2 pl-6 pr-2 py-1.5 rounded-lg text-xs text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] transition-colors"
                  >
                    <Plus className="h-3 w-3" />
                    New team
                  </Link>
                )}
              </div>
            </div>
          )}

          {!sidebarOpen && (
            <Link
              href="/teams"
              className="flex items-center justify-center px-2 py-2 rounded-lg text-sm hover:bg-[hsl(var(--muted))] text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] transition-colors"
            >
              <Users className="h-4 w-4" />
            </Link>
          )}
        </nav>

        {/* Footer */}
        <div className="border-t p-2 flex-shrink-0">
          <div className="flex items-center gap-2 px-2 py-1.5 rounded-lg">
            <Avatar name={me.display_name} size="sm" />
            {sidebarOpen && (
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium truncate leading-tight">{me.display_name}</p>
                <p className="text-xs text-[hsl(var(--muted-foreground))] truncate">{me.email}</p>
              </div>
            )}
            {sidebarOpen && (
              <button
                onClick={() => logout()}
                className="p-1.5 rounded-lg hover:bg-[hsl(var(--muted))] transition-colors flex-shrink-0"
                aria-label="Log out"
              >
                <LogOut className="h-4 w-4 text-[hsl(var(--muted-foreground))]" />
              </button>
            )}
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-y-auto bg-[hsl(var(--background))]">
        {children}
      </main>
    </div>
  );
}
