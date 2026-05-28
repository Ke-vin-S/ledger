"use client";

import { useState } from "react";
import { useTeamMembers, useAddAnonymousMember } from "@/hooks/useTeam";
import { Avatar } from "@/components/shared/Avatar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { Plus, Check, UserX } from "lucide-react";
import type { PickedMember } from "@/types/team.types";

export type { PickedMember };

type Props = {
  teamId: string;
  selected: string[];
  onChange: (ids: string[]) => void;
  /** Expose full member objects upward so SplitBuilder can render names */
  onMembersChange?: (members: PickedMember[]) => void;
};

export function MemberPicker({ teamId, selected, onChange, onMembersChange }: Props) {
  const { data: members = [] } = useTeamMembers(teamId);
  const { mutateAsync: addAnonymous, isPending: addingAnon } = useAddAnonymousMember(teamId);

  const [showAnonInput, setShowAnonInput] = useState(false);
  const [anonName, setAnonName] = useState("");
  const [anonError, setAnonError] = useState("");

  // Build a unified map of all available members
  const memberMap = new Map(members.map((m) => [m.user_id, { id: m.user_id, name: m.display_name, isAnonymous: m.identity_type === "anonymous" }]));

  function toggle(id: string) {
    const next = selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id];
    onChange(next);
    onMembersChange?.(next.map((id) => memberMap.get(id) ?? { id, name: id }));
  }

  async function handleAddAnon() {
    const name = anonName.trim();
    if (!name) { setAnonError("Name is required"); return; }
    setAnonError("");
    try {
      const created = await addAnonymous({ display_name: name });
      const newId = created.user_id;
      const newMembers: PickedMember[] = [...selected, newId].map((id) => {
        if (id === newId) return { id: newId, name, isAnonymous: true };
        return memberMap.get(id) ?? { id, name: id };
      });
      onChange([...selected, newId]);
      onMembersChange?.(newMembers);
      setAnonName("");
      setShowAnonInput(false);
    } catch {
      setAnonError("Failed to add — try again.");
    }
  }

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-2">
        {members.map((m) => {
          const isSelected = selected.includes(m.user_id);
          const isAnon = m.identity_type === "anonymous";
          return (
            <button
              key={m.user_id}
              type="button"
              onClick={() => toggle(m.user_id)}
              className={cn(
                "flex items-center gap-1.5 px-2.5 py-1.5 rounded-full border text-xs font-medium transition-all",
                isSelected
                  ? "bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] border-[hsl(var(--primary))]"
                  : "bg-[hsl(var(--background))] text-[hsl(var(--foreground))] border-[hsl(var(--border))] hover:border-[hsl(var(--primary)/0.5)]",
                isAnon && !isSelected && "border-dashed",
              )}
            >
              {isSelected ? (
                <Check className="h-3 w-3 flex-shrink-0" />
              ) : isAnon ? (
                <UserX className="h-3 w-3 flex-shrink-0 opacity-60" />
              ) : (
                <Avatar name={m.display_name} size="sm" className="h-4 w-4 text-[0.5rem]" />
              )}
              <span className="max-w-[120px] truncate">{m.display_name}</span>
              {isAnon && <span className="opacity-60 text-[0.65rem]">(anon)</span>}
            </button>
          );
        })}
      </div>

      {showAnonInput ? (
        <div className="flex gap-2 items-start">
          <div className="flex-1 space-y-1">
            <Input
              placeholder="Person's name (no account)"
              value={anonName}
              onChange={(e) => setAnonName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && (e.preventDefault(), handleAddAnon())}
              className="h-8 text-xs"
              autoFocus
            />
            {anonError && <p className="text-xs text-[hsl(var(--destructive))]">{anonError}</p>}
          </div>
          <Button type="button" size="sm" onClick={handleAddAnon} disabled={addingAnon} className="h-8 text-xs">
            {addingAnon ? "Adding…" : "Add"}
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={() => { setShowAnonInput(false); setAnonName(""); setAnonError(""); }} className="h-8 text-xs">
            Cancel
          </Button>
        </div>
      ) : (
        <button
          type="button"
          onClick={() => setShowAnonInput(true)}
          className="flex items-center gap-1.5 text-xs text-[hsl(var(--muted-foreground))] hover:text-[hsl(var(--foreground))] transition-colors"
        >
          <Plus className="h-3 w-3" />
          Add someone without account
        </button>
      )}
    </div>
  );
}
