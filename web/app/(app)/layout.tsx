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
import { PAGE_TITLES } from "@/constants/config";
import { ROUTES } from "@/constants/routes";

const navItems: { href: string; label: string; icon: React.ElementType }[] = [
  { href: ROUTES.dashboard, label: "Dashboard", icon: LayoutDashboard },
  { href: ROUTES.teams, label: "Teams", icon: Users },
  { href: ROUTES.loans, label: "Loans", icon: CreditCard },
  { href: ROUTES.notifications, label: "Notifications", icon: Bell },
  { href: ROUTES.settings, label: "Settings", icon: Settings },
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
      router.push(ROUTES.login);
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

  const pageTitle = PAGE_TITLES[pathname] ?? null;

  const isDarkMode =
    typeof window !== "undefined" && document.documentElement.classList.contains("dark");

  function toggleTheme() {
    setTheme(isDarkMode ? "light" : "dark");
  }

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar */}
      <aside
        className={cn(
          "flex flex-col bg-[hsl(var(--card))] border-r transition-all duration-200 flex-shrink-0",
          sidebarOpen ? "w-60" : "w-[3.5rem]",
        )}
      >
        {/* Header */}
        <div className="flex items-center h-14 px-3 border-b gap-2 flex-shrink-0">
          <button
            onClick={toggleSidebar}
            className="p-1.5 rounded-md hover:bg-[hsl(var(--muted))] transition-colors flex-shrink-0 text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
            aria-label="Toggle sidebar"
          >
            {sidebarOpen ? <X className="h-[1.05rem] w-[1.05rem]" /> : <Menu className="h-[1.05rem] w-[1.05rem]" />}
          </button>
          {sidebarOpen && (
            <Link
              href={ROUTES.dashboard}
              className="flex-1 text-center [font-family:var(--font-serif)] font-bold text-xl tracking-tight text-[hsl(var(--foreground))] hover:opacity-70 transition-opacity"
            >
              SplitLedger
            </Link>
          )}
          {sidebarOpen && (
            <button
              onClick={toggleTheme}
              className="p-1.5 rounded-md hover:bg-[hsl(var(--muted))] transition-colors flex-shrink-0 text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))]"
              aria-label="Toggle theme"
            >
              {isDarkMode ? <Sun className="h-[1.05rem] w-[1.05rem]" /> : <Moon className="h-[1.05rem] w-[1.05rem]" />}
            </button>
          )}
        </div>

        {/* FAB — Add Expense */}
        <div className="px-2.5 pt-3.5 pb-1.5 flex-shrink-0">
          <AddExpenseSheet>
            <button
              className={cn(
                "flex items-center gap-2 w-full rounded-lg px-3 py-2.5 text-[0.8rem] font-semibold tracking-wide transition-all duration-150",
                "bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] hover:opacity-90 active:scale-[0.98] shadow-sm",
                !sidebarOpen && "justify-center px-0",
              )}
            >
              <Plus className="h-4 w-4 flex-shrink-0" strokeWidth={2.5} />
              {sidebarOpen && <span>Add Expense</span>}
            </button>
          </AddExpenseSheet>
        </div>

        {/* Nav items */}
        <nav className="flex-1 py-1 space-y-px px-2 overflow-y-auto">
          {navItems.map(({ href, label, icon: Icon }) => {
            const isActive = pathname === href || pathname.startsWith(href + "/");
            return (
              <Link
                key={href}
                href={href as never}
                className={cn(
                  "flex items-center gap-3 py-2 text-[0.82rem] font-medium transition-all duration-100 group relative rounded-md",
                  sidebarOpen ? "px-2.5" : "justify-center px-0",
                  isActive
                    ? "text-[hsl(var(--primary))] bg-[hsl(var(--accent))]"
                    : "text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--muted))] hover:text-[hsl(var(--foreground))]",
                )}
              >
                {/* Active left border */}
                {isActive && sidebarOpen && (
                  <span className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-0.5 rounded-full bg-[hsl(var(--primary))]" />
                )}
                <div className="relative flex-shrink-0">
                  <Icon className="h-[1.05rem] w-[1.05rem]" />
                  {href === ROUTES.notifications && unreadCount > 0 && (
                    <span className="absolute -top-1 -right-1.5 h-3.5 w-3.5 rounded-full bg-[hsl(var(--destructive))] text-[hsl(var(--destructive-foreground))] text-[7px] flex items-center justify-center font-bold">
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
            <div className="pt-4 pb-1">
              <p className="px-2.5 pb-1.5 text-[0.7rem] font-semibold tracking-[0.1em] uppercase text-[hsl(var(--muted-foreground))]">
                Teams
              </p>
              <div className="space-y-px">
                {teams?.slice(0, 6).map((team) => {
                  const isActive = pathname.startsWith(`/teams/${team.id}`);
                  return (
                    <Link
                      key={team.id}
                      href={ROUTES.team(team.id) as never}
                      className={cn(
                        "flex items-center gap-2.5 pl-2.5 pr-2 py-1.5 rounded-md text-[0.82rem] transition-all duration-100 truncate relative",
                        isActive
                          ? "text-[hsl(var(--primary))] bg-[hsl(var(--accent))] font-medium"
                          : "text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--muted))] hover:text-[hsl(var(--foreground))]",
                      )}
                    >
                      {isActive && (
                        <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-full bg-[hsl(var(--primary))]" />
                      )}
                      <span className={cn(
                        "h-1.5 w-1.5 rounded-full flex-shrink-0",
                        isActive ? "bg-[hsl(var(--primary))]" : "bg-[hsl(var(--muted-foreground))] opacity-50",
                      )} />
                      <span className="truncate">{team.name}</span>
                    </Link>
                  );
                })}
                <Link
                  href={ROUTES.teams}
                  className="flex items-center gap-2.5 pl-2.5 pr-2 py-1.5 rounded-md text-[0.78rem] text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--primary))] transition-colors"
                >
                  <Plus className="h-3 w-3" />
                  {(teams?.length ?? 0) === 0 ? "Create a team" : "All teams"}
                </Link>
              </div>
            </div>
          )}

          {!sidebarOpen && (
            <Link
              href={ROUTES.teams}
              className="flex items-center justify-center py-2 rounded-md text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--muted))] hover:text-[hsl(var(--foreground))] transition-colors"
            >
              <Users className="h-[1.05rem] w-[1.05rem]" />
            </Link>
          )}
        </nav>

        {/* Footer */}
        <div className="border-t px-2.5 py-2.5 flex-shrink-0">
          <div className="flex items-center gap-2.5">
            <Avatar name={me.display_name} size="sm" />
            {sidebarOpen && (
              <div className="flex-1 min-w-0">
                <p className="text-[0.82rem] font-semibold truncate leading-tight">{me.display_name}</p>
                <p className="text-[0.72rem] text-[hsl(var(--muted-foreground))] truncate mt-0.5">{me.email}</p>
              </div>
            )}
            {sidebarOpen && (
              <button
                onClick={() => logout()}
                className="p-1.5 rounded-md hover:bg-[hsl(var(--muted))] transition-colors flex-shrink-0"
                aria-label="Log out"
              >
                <LogOut className="h-[1.05rem] w-[1.05rem] text-[hsl(var(--muted-foreground))]" />
              </button>
            )}
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 flex flex-col overflow-hidden bg-[hsl(var(--background))]">
        <div className="h-14 border-b flex items-center px-8 flex-shrink-0">
          {pageTitle && (
            <h1 className="text-xl font-bold tracking-tight">{pageTitle}</h1>
          )}
        </div>
        <div className="flex-1 overflow-y-auto">
          {children}
        </div>
      </main>
    </div>
  );
}
