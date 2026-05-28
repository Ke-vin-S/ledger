"use client";

import { create } from "zustand";

type Theme = "light" | "dark" | "system";

type Modal =
  | { type: "create-expense"; teamId: string }
  | { type: "create-team" }
  | { type: "invite-member"; teamId: string }
  | { type: "record-settlement"; expenseId: string }
  | { type: "expense-detail"; expenseId: string };

type UIStore = {
  sidebarOpen: boolean;
  setSidebarOpen: (open: boolean) => void;
  toggleSidebar: () => void;

  theme: Theme;
  setTheme: (theme: Theme) => void;

  modals: Modal[];
  openModal: (modal: Modal) => void;
  closeModal: () => void;
  closeAllModals: () => void;
};

export const useUIStore = create<UIStore>((set) => ({
  sidebarOpen: true,
  setSidebarOpen: (open) => set({ sidebarOpen: open }),
  toggleSidebar: () => set((s) => ({ sidebarOpen: !s.sidebarOpen })),

  theme: "system",
  setTheme: (theme) => {
    set({ theme });
    const root = document.documentElement;
    if (theme === "dark") {
      root.classList.add("dark");
    } else if (theme === "light") {
      root.classList.remove("dark");
    } else {
      const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      root.classList.toggle("dark", prefersDark);
    }
  },

  modals: [],
  openModal: (modal) => set((s) => ({ modals: [...s.modals, modal] })),
  closeModal: () => set((s) => ({ modals: s.modals.slice(0, -1) })),
  closeAllModals: () => set({ modals: [] }),
}));
