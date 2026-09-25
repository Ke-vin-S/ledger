"use client";
import { useState } from "react";
import { usePathname } from "next/navigation";
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
} from "lucide-react";
import { useMe, useLogout } from "@/hooks/useAuth";
import { useTeams } from "@/hooks/useTeam";
import { useUIStore } from "@/store/ui";
import { useNotifications } from "@/hooks/useNotifications";
import { Avatar } from "@/components/shared/Avatar";
import { AddExpenseSheet } from "@/components/expense/AddExpenseSheet";
import { BottomNav } from "@/components/shared/BottomNav";
import { Sheet, SheetContent } from "@/components/ui/sheet";
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
  const pathname = usePathname();
  const { data: me, isLoading } = useMe();
  const { data: teams } = useTeams();
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false);
  const { sidebarOpen, toggleSidebar } = useUIStore();
  const { mutate: logout } = useLogout();
  const { data: notifications } = useNotifications(true);

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 rounded-full bg-primary opacity-20 animate-pulse" />
          <p className="text-muted-foreground text-sm">Loading…</p>
        </div>
      </div>
    );
  }

  if (!me) {
    return (
      <div className="min-h-screen flex items-center justify-center px-6 text-center">
        <div className="space-y-3">
          <p className="font-medium">Your session has ended.</p>
          <p className="text-sm text-muted-foreground">
            Sign in again to continue.
          </p>
          <a
            href="/login"
            className="inline-flex h-10 items-center rounded-md bg-primary px-4 text-sm font-medium text-primary-foreground"
          >
            Go to sign in
          </a>
        </div>
      </div>
    );
  }

  const unreadCount = notifications?.items.length ?? 0;

  const pageTitle = PAGE_TITLES[pathname] ?? null;

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Sidebar — desktop only */}
      <aside
        className={cn(
          "hidden md:flex flex-col bg-card border-r transition-all duration-200 flex-shrink-0",
          sidebarOpen ? "w-60" : "w-14",
        )}
      >
        {/* Header */}
        <div className="flex items-center h-14 px-3 border-b gap-2 flex-shrink-0">
          <button
            type="button"
            onClick={toggleSidebar}
            className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label={sidebarOpen ? "Collapse sidebar" : "Expand sidebar"}
          >
            {sidebarOpen ? (
              <X className="h-5 w-5" />
            ) : (
              <Menu className="h-5 w-5" />
            )}
          </button>
          {sidebarOpen && (
            <Link
              href={ROUTES.dashboard}
              className="flex-1 text-center font-display font-bold text-xl tracking-tight text-foreground hover:opacity-70 transition-opacity"
            >
              SplitLedger
            </Link>
          )}
        </div>

        {/* FAB — Add Expense */}
        <div className="px-2.5 pt-3.5 pb-1.5 flex-shrink-0">
          <AddExpenseSheet>
            <button
              type="button"
              className={cn(
                "flex items-center gap-2 w-full rounded-lg px-3 py-2.5 text-sm font-semibold tracking-wide transition-all duration-150",
                "bg-primary text-primary-foreground hover:opacity-90 active:scale-[0.98] shadow-sm",
                !sidebarOpen && "h-9 min-h-9 justify-center px-0",
              )}
              aria-label="Add expense"
            >
              <Plus className="h-4 w-4 flex-shrink-0" strokeWidth={2.5} />
              {sidebarOpen && <span>Add Expense</span>}
            </button>
          </AddExpenseSheet>
        </div>

        {/* Nav items */}
        <nav className="flex-1 py-1 space-y-px px-2 overflow-y-auto">
          {navItems.map(({ href, label, icon: Icon }) => {
            const isActive =
              pathname === href || pathname.startsWith(href + "/");
            return (
              <Link
                key={href}
                href={href as never}
                className={cn(
                  "flex items-center gap-3 py-2 text-sm font-medium transition-all duration-100 group relative rounded-md",
                  sidebarOpen ? "px-2.5" : "justify-center px-0",
                  isActive
                    ? "text-primary bg-accent"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                )}
              >
                {/* Active left border */}
                {isActive && sidebarOpen && (
                  <span className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-0.5 rounded-full bg-primary" />
                )}
                <div className="relative flex-shrink-0">
                  <Icon className="h-5 w-5" />
                  {href === ROUTES.notifications && unreadCount > 0 && (
                    <span
                      className="absolute -top-1 -right-1.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-destructive px-1 text-[0.5rem] font-bold text-destructive-foreground"
                      aria-label={`${unreadCount} unread notification${unreadCount === 1 ? "" : "s"}`}
                    >
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
              <p className="px-2.5 pb-1.5 text-xs font-semibold tracking-[0.1em] uppercase text-muted-foreground">
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
                        "flex items-center gap-2.5 pl-2.5 pr-2 py-1.5 rounded-md text-sm transition-all duration-100 truncate relative",
                        isActive
                          ? "text-primary bg-accent font-medium"
                          : "text-muted-foreground hover:bg-muted hover:text-foreground",
                      )}
                    >
                      {isActive && (
                        <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-full bg-primary" />
                      )}
                      <span
                        className={cn(
                          "h-1.5 w-1.5 rounded-full flex-shrink-0",
                          isActive
                            ? "bg-primary"
                            : "bg-muted-foreground opacity-50",
                        )}
                      />
                      <span className="truncate">{team.name}</span>
                    </Link>
                  );
                })}
                <Link
                  href={ROUTES.teams}
                  className="flex items-center gap-2.5 pl-2.5 pr-2 py-1.5 rounded-md text-sm text-muted-foreground hover:text-primary transition-colors"
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
              aria-label="All teams"
              className="flex h-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            >
              <Users className="h-5 w-5" />
            </Link>
          )}
        </nav>

        {/* Footer */}
        <div className="border-t px-2.5 py-2.5 flex-shrink-0">
          <div className="flex items-center gap-2.5">
            <Avatar
              name={me.display_name}
              src={me.avatar_url ?? undefined}
              size="sm"
            />
            {sidebarOpen && (
              <div className="flex-1 min-w-0">
                <p className="text-sm font-semibold truncate leading-tight">
                  {me.display_name}
                </p>
                <p className="text-xs text-muted-foreground truncate mt-0.5">
                  {me.email}
                </p>
              </div>
            )}
            {sidebarOpen && (
              <button
                type="button"
                onClick={() => logout()}
                className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md transition-colors hover:bg-muted"
                aria-label="Log out"
              >
                <LogOut className="h-5 w-5 text-muted-foreground" />
              </button>
            )}
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 flex flex-col overflow-hidden bg-background">
        {/* Mobile header */}
        <div className="md:hidden h-14 border-b flex items-center px-4 gap-3 flex-shrink-0 bg-card">
          <button
            type="button"
            onClick={() => setMobileDrawerOpen(true)}
            className="flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="Open navigation"
          >
            <Menu className="h-5 w-5" />
          </button>
          <Link
            href={ROUTES.dashboard}
            className="flex-1 font-display font-bold text-lg tracking-tight text-foreground hover:opacity-70 transition-opacity"
          >
            SplitLedger
          </Link>
        </div>

        {/* Desktop header */}
        <div className="hidden md:flex h-14 border-b items-center px-8 flex-shrink-0">
          {pageTitle && (
            <h1 className="text-xl font-bold tracking-tight">{pageTitle}</h1>
          )}
        </div>

        <div className="flex-1 overflow-y-auto pb-[var(--bottom-nav-h)] md:pb-0">
          {children}
        </div>
      </main>

      {/* Mobile nav drawer */}
      <Sheet open={mobileDrawerOpen} onOpenChange={setMobileDrawerOpen}>
        <SheetContent
          side="left"
          title="Navigation"
          description="Navigate to your SplitLedger pages and add a new expense."
          className="w-72 max-w-[85vw] flex flex-col"
        >
          {/* FAB */}
          <div className="px-1 pt-1 pb-3">
            <AddExpenseSheet>
              <button
                type="button"
                className="flex items-center gap-2 w-full rounded-lg px-3 py-2.5 text-sm font-semibold tracking-wide bg-primary text-primary-foreground hover:opacity-90 active:scale-[0.98] shadow-sm"
                onClick={() => setMobileDrawerOpen(false)}
              >
                <Plus className="h-4 w-4 flex-shrink-0" strokeWidth={2.5} />
                <span>Add Expense</span>
              </button>
            </AddExpenseSheet>
          </div>

          {/* Nav items */}
          <nav className="space-y-px">
            {navItems.map(({ href, label, icon: Icon }) => {
              const isActive =
                pathname === href || pathname.startsWith(href + "/");
              return (
                <Link
                  key={href}
                  href={href as never}
                  onClick={() => setMobileDrawerOpen(false)}
                  className={cn(
                    "flex items-center gap-3 py-2 px-2.5 text-sm font-medium rounded-md transition-colors relative",
                    isActive
                      ? "text-primary bg-accent"
                      : "text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  {isActive && (
                    <span className="absolute left-0 top-1/2 -translate-y-1/2 h-5 w-0.5 rounded-full bg-primary" />
                  )}
                  <div className="relative flex-shrink-0">
                    <Icon className="h-5 w-5" />
                    {href === ROUTES.notifications && unreadCount > 0 && (
                      <span
                        className="absolute -top-1 -right-1.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-destructive px-1 text-[0.5rem] font-bold text-destructive-foreground"
                        aria-label={`${unreadCount} unread notification${unreadCount === 1 ? "" : "s"}`}
                      >
                        {unreadCount > 9 ? "9+" : unreadCount}
                      </span>
                    )}
                  </div>
                  <span>{label}</span>
                </Link>
              );
            })}
          </nav>

          {/* Teams sub-list */}
          {(teams?.length ?? 0) > 0 && (
            <div className="pt-4 pb-1 border-t mt-4">
              <p className="px-2.5 pb-1.5 text-xs font-semibold tracking-[0.1em] uppercase text-muted-foreground">
                Teams
              </p>
              <div className="space-y-px">
                {teams?.slice(0, 6).map((team) => {
                  const isActive = pathname.startsWith(`/teams/${team.id}`);
                  return (
                    <Link
                      key={team.id}
                      href={ROUTES.team(team.id) as never}
                      onClick={() => setMobileDrawerOpen(false)}
                      className={cn(
                        "flex items-center gap-2.5 pl-2.5 pr-2 py-1.5 rounded-md text-sm transition-colors truncate relative",
                        isActive
                          ? "text-primary bg-accent font-medium"
                          : "text-muted-foreground hover:bg-muted hover:text-foreground",
                      )}
                    >
                      {isActive && (
                        <span className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-0.5 rounded-full bg-primary" />
                      )}
                      <span
                        className={cn(
                          "h-1.5 w-1.5 rounded-full flex-shrink-0",
                          isActive
                            ? "bg-primary"
                            : "bg-muted-foreground opacity-50",
                        )}
                      />
                      <span className="truncate">{team.name}</span>
                    </Link>
                  );
                })}
                <Link
                  href={ROUTES.teams}
                  onClick={() => setMobileDrawerOpen(false)}
                  className="flex items-center gap-2.5 pl-2.5 pr-2 py-1.5 rounded-md text-sm text-muted-foreground hover:text-primary transition-colors"
                >
                  <Plus className="h-3 w-3" />
                  All teams
                </Link>
              </div>
            </div>
          )}

          {/* Footer */}
          <div className="sticky bottom-0 -mx-6 -mb-5 px-6 py-3 border-t bg-card mt-6">
            <div className="flex items-center gap-2.5">
              <Avatar
                name={me.display_name}
                src={me.avatar_url ?? undefined}
                size="sm"
              />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-semibold truncate leading-tight">
                  {me.display_name}
                </p>
                <p className="text-xs text-muted-foreground truncate mt-0.5">
                  {me.email}
                </p>
              </div>
              <button
                type="button"
                onClick={() => logout()}
                className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md transition-colors hover:bg-muted"
                aria-label="Log out"
              >
                <LogOut className="h-5 w-5 text-muted-foreground" />
              </button>
            </div>
          </div>
        </SheetContent>
      </Sheet>

      <BottomNav />
    </div>
  );
}
