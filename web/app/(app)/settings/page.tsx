"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Monitor, Moon, Sun } from "lucide-react";
import {
  useMe,
  useUpdateCurrencyPref,
  useUpdateProfile,
} from "@/hooks/useAuth";
import { ApiRequestError } from "@/lib/api";
import { CURRENCIES } from "@/constants/config";
import { useUIStore } from "@/store/ui";
import { PageHeader } from "@/components/shared/PageHeader";
import { QueryBoundary } from "@/components/query-boundary";
import { Avatar } from "@/components/shared/Avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useToast } from "@/components/ui/toast";
import type { User } from "@/types/user.types";

const profileSchema = z.object({
  display_name: z.string().min(1, "Name is required").max(80),
});
type ProfileValues = z.infer<typeof profileSchema>;

function ProfileSection({ user }: { user: User }) {
  const { mutateAsync: updateProfile, isPending } = useUpdateProfile();
  const { toast } = useToast();
  const [serverError, setServerError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    values: { display_name: user.display_name },
  });

  async function onSubmit(data: ProfileValues) {
    setServerError(null);
    try {
      await updateProfile(data);
      toast({ title: "Profile updated", variant: "success" });
    } catch (err) {
      setServerError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to update profile.",
      );
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="mb-6 flex items-center gap-4">
          <Avatar
            name={user.display_name}
            src={user.avatar_url ?? undefined}
            size="lg"
          />
          <div>
            <p className="font-semibold">{user.display_name}</p>
            <p className="text-sm text-muted-foreground">{user.email}</p>
          </div>
        </div>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {serverError ? (
            <p
              role="alert"
              className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
            >
              {serverError}
            </p>
          ) : null}
          <div className="space-y-1.5">
            <Label htmlFor="settings-display-name">Display name</Label>
            <Input
              id="settings-display-name"
              aria-invalid={errors.display_name ? true : undefined}
              {...register("display_name")}
            />
            {errors.display_name ? (
              <p className="text-xs text-destructive">
                {errors.display_name.message}
              </p>
            ) : null}
          </div>
          <Button type="submit" disabled={isPending}>
            {isPending ? "Saving…" : "Save profile"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function CurrencySection({ user }: { user: User }) {
  const { mutateAsync: updateCurrencyPref, isPending } =
    useUpdateCurrencyPref();
  const { toast } = useToast();
  const [selected, setSelected] = useState(user.currency_pref);
  const [error, setError] = useState<string | null>(null);

  async function saveCurrency() {
    if (selected === user.currency_pref) return;
    setError(null);
    try {
      await updateCurrencyPref(selected);
      toast({ title: "Default currency updated", variant: "success" });
    } catch (err) {
      setError(
        err instanceof ApiRequestError
          ? err.error.message
          : "Failed to update currency.",
      );
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Preferences</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-1.5">
          <Label htmlFor="settings-currency">Default currency</Label>
          <Select value={selected} onValueChange={setSelected}>
            <SelectTrigger id="settings-currency">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {CURRENCIES.map(({ code, label }) => (
                <SelectItem key={code} value={code}>
                  {label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className="text-xs text-muted-foreground">
            Used by default when adding expenses. You can still change it per
            expense.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button
            type="button"
            onClick={saveCurrency}
            disabled={isPending || selected === user.currency_pref}
          >
            {isPending ? "Saving…" : "Save currency"}
          </Button>
          <span className="text-xs text-muted-foreground">
            Current: {user.currency_pref}
          </span>
        </div>
        <div className="space-y-2">
          <Label>Theme</Label>
          <div
            className="grid grid-cols-3 gap-2"
            role="group"
            aria-label="Theme"
          >
            <ThemeButton value="light" label="Light" icon={Sun} />
            <ThemeButton value="dark" label="Dark" icon={Moon} />
            <ThemeButton value="system" label="System" icon={Monitor} />
          </div>
        </div>
        {error ? (
          <p
            role="alert"
            className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive"
          >
            {error}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}

function ThemeButton({
  value,
  label,
  icon: Icon,
}: {
  value: "light" | "dark" | "system";
  label: string;
  icon: typeof Sun;
}) {
  const theme = useUIStore((state) => state.theme);
  const setTheme = useUIStore((state) => state.setTheme);
  return (
    <Button
      type="button"
      variant={theme === value ? "default" : "outline"}
      aria-pressed={theme === value}
      onClick={() => setTheme(value)}
      className="w-full"
    >
      <Icon className="h-4 w-4" aria-hidden="true" />
      {label}
    </Button>
  );
}

export default function SettingsPage() {
  const query = useMe();
  return (
    <main className="mx-auto max-w-3xl space-y-8 p-4 md:p-8">
      <PageHeader
        eyebrow="Your account"
        title="Settings"
        description="Keep your profile, defaults, and viewing preferences in one place."
      />
      <QueryBoundary {...query} isEmpty={() => false} className="min-h-64">
        {(user) => (
          <div className="grid gap-6 md:grid-cols-2">
            <ProfileSection user={user} />
            <CurrencySection user={user} />
          </div>
        )}
      </QueryBoundary>
    </main>
  );
}
