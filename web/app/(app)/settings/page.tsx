"use client";

import { useState } from "react";
import { useMe } from "@/hooks/useAuth";
import { api, ApiRequestError } from "@/lib/api";
import { useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Skeleton } from "@/components/shared/Skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Avatar } from "@/components/shared/Avatar";

const CURRENCIES = [
  { code: "LKR", label: "LKR — Sri Lanka Rupee" },
  { code: "USD", label: "USD — US Dollar" },
  { code: "EUR", label: "EUR — Euro" },
  { code: "GBP", label: "GBP — British Pound" },
  { code: "INR", label: "INR — Indian Rupee" },
  { code: "SGD", label: "SGD — Singapore Dollar" },
  { code: "AUD", label: "AUD — Australian Dollar" },
];

const profileSchema = z.object({
  display_name: z.string().min(1, "Name is required").max(80),
});
type ProfileValues = z.infer<typeof profileSchema>;

function ProfileSection() {
  const { data: me, isLoading } = useMe();
  const qc = useQueryClient();
  const [serverError, setServerError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const { register, handleSubmit, formState: { errors, isSubmitting } } = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    values: { display_name: me?.display_name ?? "" },
  });

  if (isLoading) return <Skeleton className="h-40" />;

  async function onSubmit(data: ProfileValues) {
    setServerError(null);
    setSaved(false);
    try {
      await api.patch("/users/me", data);
      await qc.invalidateQueries({ queryKey: ["users", "me"] });
      setSaved(true);
    } catch (err) {
      if (err instanceof ApiRequestError) setServerError(err.error.message);
      else setServerError("Failed to update profile.");
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Profile</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex items-center gap-4 mb-5">
          <Avatar name={me?.display_name ?? "?"} size="lg" />
          <div>
            <p className="font-medium">{me?.display_name}</p>
            <p className="text-sm text-[hsl(var(--muted-foreground))]">{me?.email}</p>
          </div>
        </div>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          {serverError && (
            <p className="text-sm text-[hsl(var(--destructive))]">{serverError}</p>
          )}
          {saved && (
            <p className="text-sm text-[hsl(var(--positive))]">Saved!</p>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="display_name">Display name</Label>
            <Input id="display_name" {...register("display_name")} />
            {errors.display_name && (
              <p className="text-xs text-[hsl(var(--destructive))]">
                {errors.display_name.message}
              </p>
            )}
          </div>
          <Button type="submit" size="sm" disabled={isSubmitting}>
            {isSubmitting ? "Saving…" : "Save changes"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

function CurrencySection() {
  const { data: me, isLoading } = useMe();
  const qc = useQueryClient();
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string>("");

  const currentCurrency = me?.currency_pref ?? "LKR";

  if (isLoading) return <Skeleton className="h-28" />;

  async function handleSave() {
    if (!selected || selected === currentCurrency) return;
    setSaving(true);
    setSaved(false);
    setServerError(null);
    try {
      await api.patch("/users/me", { currency_pref: selected });
      await qc.invalidateQueries({ queryKey: ["users", "me"] });
      setSaved(true);
      setSelected("");
    } catch (err) {
      if (err instanceof ApiRequestError) setServerError(err.error.message);
      else setServerError("Failed to update currency.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Default currency</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-[hsl(var(--muted-foreground))]">
          Used as the default when adding expenses. You can always change it per expense.
        </p>
        {serverError && (
          <p className="text-sm text-[hsl(var(--destructive))]">{serverError}</p>
        )}
        {saved && (
          <p className="text-sm text-[hsl(var(--positive))]">Currency preference saved!</p>
        )}
        <div className="flex items-center gap-3">
          <div className="flex-1 space-y-1.5">
            <Label htmlFor="currency_pref">Currency</Label>
            <select
              id="currency_pref"
              className="w-full h-9 rounded-md border border-[hsl(var(--input))] bg-[hsl(var(--background))] px-3 text-sm focus:outline-none focus:ring-1 focus:ring-[hsl(var(--ring))]"
              defaultValue={currentCurrency}
              onChange={(e) => setSelected(e.target.value)}
            >
              {CURRENCIES.map(({ code, label }) => (
                <option key={code} value={code}>{label}</option>
              ))}
            </select>
          </div>
          <Button
            size="sm"
            className="mt-6"
            disabled={saving || !selected || selected === currentCurrency}
            onClick={handleSave}
          >
            {saving ? "Saving…" : "Save"}
          </Button>
        </div>
        <p className="text-xs text-[hsl(var(--muted-foreground))]">
          Current: <span className="font-medium text-[hsl(var(--foreground))]">{currentCurrency}</span>
        </p>
      </CardContent>
    </Card>
  );
}

export default function SettingsPage() {
  return (
    <div className="p-8 space-y-6 max-w-2xl">
      <div>
        <h1 className="text-3xl font-bold">Settings</h1>
        <p className="text-sm text-[hsl(var(--muted-foreground))] mt-1">
          Manage your account preferences
        </p>
      </div>
      <ProfileSection />
      <CurrencySection />
    </div>
  );
}
