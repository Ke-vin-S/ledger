"use client";

import { useState } from "react";
import { useMe, useUpdateProfile, useUpdateCurrencyPref } from "@/hooks/useAuth";
import { ApiRequestError } from "@/lib/api";
import { CURRENCIES } from "@/constants/config";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { Skeleton } from "@/components/shared/Skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Avatar } from "@/components/shared/Avatar";

const profileSchema = z.object({
  display_name: z.string().min(1, "Name is required").max(80),
});
type ProfileValues = z.infer<typeof profileSchema>;

function ProfileSection() {
  const { data: me, isLoading } = useMe();
  const { mutateAsync: updateProfile } = useUpdateProfile();
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
      await updateProfile(data);
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
          <Avatar name={me?.display_name ?? "?"} src={me?.avatar_url ?? undefined} size="lg" />
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
  const { mutateAsync: updateCurrencyPref, isPending: saving } = useUpdateCurrencyPref();
  const [saved, setSaved] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string>("");

  const currentCurrency = me?.currency_pref ?? "LKR";

  if (isLoading) return <Skeleton className="h-28" />;

  async function handleSave() {
    if (!selected || selected === currentCurrency) return;
    setSaved(false);
    setServerError(null);
    try {
      await updateCurrencyPref(selected);
      setSaved(true);
      setSelected("");
    } catch (err) {
      if (err instanceof ApiRequestError) setServerError(err.error.message);
      else setServerError("Failed to update currency.");
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
            <Label>Currency</Label>
            <Select defaultValue={currentCurrency} onValueChange={setSelected}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CURRENCIES.map(({ code, label }) => (
                  <SelectItem key={code} value={code}>{label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
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
    <div className="p-4 md:p-8 space-y-6 max-w-2xl">
      <ProfileSection />
      <CurrencySection />
    </div>
  );
}
