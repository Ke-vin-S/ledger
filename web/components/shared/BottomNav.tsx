"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  LayoutDashboard,
  Users,
  CreditCard,
  Bell,
  Settings,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { ROUTES } from "@/constants/routes";
import { useNotifications } from "@/hooks/useNotifications";

const navItems = [
  { href: ROUTES.dashboard,     label: "Dashboard", icon: LayoutDashboard },
  { href: ROUTES.teams,         label: "Teams",     icon: Users            },
  { href: ROUTES.loans,         label: "Loans",     icon: CreditCard       },
  { href: ROUTES.notifications, label: "Alerts",    icon: Bell             },
  { href: ROUTES.settings,      label: "Settings",  icon: Settings         },
] as const;

export function BottomNav() {
  const pathname = usePathname();
  const { data: notifications } = useNotifications(true);
  const unreadCount = notifications?.items.length ?? 0;

  return (
    <nav
      className="fixed bottom-0 left-0 right-0 z-40 md:hidden bg-[hsl(var(--card))] border-t pb-safe"
      aria-label="Main navigation"
    >
      <div className="flex h-16 items-stretch">
        {navItems.map(({ href, label, icon: Icon }) => {
          const isActive = pathname === href || pathname.startsWith(href + "/");
          const isNotif = href === ROUTES.notifications;
          return (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex flex-1 flex-col items-center justify-center gap-0.5 text-[0.6rem] font-medium tracking-wide transition-colors relative",
                isActive
                  ? "text-[hsl(var(--primary))]"
                  : "text-[hsl(var(--muted-foreground))]",
              )}
            >
              {isActive && (
                <span className="absolute top-0 left-1/2 -translate-x-1/2 h-0.5 w-8 rounded-full bg-[hsl(var(--primary))]" />
              )}
              <div className="relative">
                <Icon
                  className="h-5 w-5"
                  strokeWidth={isActive ? 2.5 : 1.75}
                />
                {isNotif && unreadCount > 0 && (
                  <span className="absolute -top-1 -right-1.5 h-3.5 w-3.5 rounded-full bg-[hsl(var(--destructive))] text-[hsl(var(--destructive-foreground))] text-[7px] flex items-center justify-center font-bold">
                    {unreadCount > 9 ? "9+" : unreadCount}
                  </span>
                )}
              </div>
              <span>{label}</span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}
