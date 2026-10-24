"use client";

import Link from "next/link";
import { useState } from "react";

import { ChimeToggle } from "@/components/settings/ChimeToggle";
import { Button } from "@/components/ui/Button";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import { Segmented } from "@/components/ui/Select";
import { ACCOUNT_NAV } from "@/lib/nav";
import type { SessionUser } from "@/lib/session";
import { useTheme } from "@/lib/theme";
import { useInstallPrompt } from "@/lib/useInstallPrompt";

/**
 * Who you are signed in as, and the two things that belong to this device.
 *
 * One control opens it on every surface, so account never depends on having a
 * nav sheet — a member with a tab bar and an advanced operator with a rail
 * reach it the same way.
 *
 * Sign-out lives here rather than in the header or the rail, and that is the
 * point of the sheet. On a phone a standing sign-out button sits one mis-tap
 * from a destination, and signing out clears every tab's place — which is
 * cheap to say and expensive to discover.
 */
/** What each of an operator's own pages is for, under its link. */
const OWN_NOTES: Record<string, string> = {
  "/me": "What you can use, and asking for more",
  "/storage": "Your password and folders",
};

export function AccountSheet({ session }: { session: SessionUser }) {
  const [open, setOpen] = useState(false);
  const { theme, setTheme } = useTheme();
  const installer = useInstallPrompt();

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="flex min-h-[44px] items-center gap-[9px] rounded-pill pl-1 pr-1 text-left control press-scale hover:bg-[var(--hover)] tablet:pr-3"
      >
        <span
          aria-hidden
          className="avatar-fill flex h-[34px] w-[34px] flex-none items-center justify-center rounded-pill text-[12.5px] font-semibold text-ink/70"
        >
          {session.avatar}
        </span>
        {/* The name is orientation, not identification, so it goes when the
            room is narrow. Email is the fallback and never the id: if every
            naming source came up empty, an address still names a human. */}
        <span className="hidden max-w-[180px] truncate text-[13.5px] text-muted tablet:block">
          {session.name || session.email}
        </span>
        <span className="sr-only">Account</span>
      </button>

      <Modal open={open} onClose={() => setOpen(false)} size="sm" labelledBy="account-sheet-title">
        <ModalHeader title="Account" titleId="account-sheet-title" />

        <div className="flex flex-col gap-3.5 px-6 pb-2">
          <div className="flex items-center gap-3">
            <span
              aria-hidden
              className="avatar-fill flex h-[44px] w-[44px] flex-none items-center justify-center rounded-pill text-[15px] font-semibold text-ink/70"
            >
              {session.avatar}
            </span>
            <span className="min-w-0">
              <span className="block truncate text-[15px] font-semibold">
                {session.name || session.email}
              </span>
              {session.name && (
                <span className="block truncate text-[13px] text-muted">{session.email}</span>
              )}
            </span>
          </div>

          {/* An operator's own pages, between two hairlines: going somewhere,
              so violet links. A member has these in their rail already. */}
          {session.role !== "user" && (
            <div className="flex flex-col border-y border-line py-1.5">
              {ACCOUNT_NAV.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  onClick={() => setOpen(false)}
                  className="-mx-2 flex min-h-[44px] flex-col justify-center gap-0.5 rounded-inner px-2 py-1.5 hover-tint"
                >
                  <span className="text-[14.5px] font-semibold text-accent-text">{item.label} →</span>
                  <span className="text-[13px] text-muted">{OWN_NOTES[item.href]}</span>
                </Link>
              ))}
            </div>
          )}

          {/* A setting about this device, in a sheet about this device. The
              header keeps its own toggle where there is room for one. */}
          <div className="flex items-center justify-between gap-3 rounded-inner border border-line py-1.5 pl-4 pr-2 text-[14.5px]">
            <span>Appearance</span>
            <Segmented
              label="Appearance"
              size="sm"
              value={theme ?? "dark"}
              onChange={setTheme}
              options={[
                { value: "dark", label: "Dark" },
                { value: "light", label: "Light" },
              ]}
            />
          </div>

          {/* Another setting about this device: the chime is a browser
              preference, so it sits beside Appearance rather than among the
              org-wide automation policy it used to share a box with. */}
          <div className="flex flex-col gap-2.5 rounded-inner border border-line px-4 py-3 text-[14.5px]">
            <span className="flex flex-col gap-0.5">
              <span>Sound when drift is found</span>
              <span className="text-[13px] text-muted">In this browser only</span>
            </span>
            <ChimeToggle />
          </div>

          {/* Offered here rather than as a banner on arrival. A first-load
              prompt interrupts somebody who came to read one thing and gets
              dismissed by reflex; this is where a member already is when they
              are thinking about the app rather than about a machine they want
              to book. Absent entirely when there is nothing to offer. */}
          {installer && (
            <button
              type="button"
              onClick={() => void installer.install()}
              className="flex min-h-[44px] items-center justify-between rounded-inner border border-line px-4 text-[14.5px] control press-scale hover:bg-[var(--hover)]"
            >
              <span>Install as an app</span>
              <span className="text-muted">Adds a home-screen icon</span>
            </button>
          )}
        </div>

        {/* Below a hairline and last, because it is the one thing here that
            ends the session rather than adjusting it. */}
        <div className="mt-3 border-t border-line px-6 pb-[22px] pt-4">
          <form action="/auth/logout" method="post">
            <Button type="submit" variant="outline" className="w-full">
              Sign out
            </Button>
          </form>
          <p className="mt-2.5 text-[12.5px] leading-[1.5] text-faint">
            Signing out signs you out in every open tab. You stay signed in for weeks otherwise,
            so there is no need to do this at the end of each visit.
          </p>
        </div>
      </Modal>
    </>
  );
}
