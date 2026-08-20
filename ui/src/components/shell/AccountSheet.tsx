"use client";

import { useState } from "react";

import { Modal, ModalHeader } from "@/components/ui/Modal";
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
export function AccountSheet({ session }: { session: SessionUser }) {
  const [open, setOpen] = useState(false);
  const { theme, toggle } = useTheme();
  const installer = useInstallPrompt();

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="flex min-h-[44px] items-center gap-[9px] rounded-pill pl-1 pr-1 text-left motion-press tablet:pr-3"
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

          {/* A setting about this device, in a sheet about this device. The
              header keeps its own toggle where there is room for one. */}
          <button
            type="button"
            onClick={toggle}
            className="flex min-h-[44px] items-center justify-between rounded-inner border border-line px-4 text-[14.5px] motion-press"
          >
            <span>Appearance</span>
            <span className="text-muted">{theme === "dark" ? "Dark" : "Light"}</span>
          </button>

          {/* Offered here rather than as a banner on arrival. A first-load
              prompt interrupts somebody who came to read one thing and gets
              dismissed by reflex; this is where a member already is when they
              are thinking about the app rather than about a machine they want
              to book. Absent entirely when there is nothing to offer. */}
          {installer && (
            <button
              type="button"
              onClick={() => void installer.install()}
              className="flex min-h-[44px] items-center justify-between rounded-inner border border-line px-4 text-[14.5px] motion-press"
            >
              <span>Install on this phone</span>
              <span className="text-muted">Adds an icon</span>
            </button>
          )}
        </div>

        {/* Below a hairline and last, because it is the one thing here that
            ends the session rather than adjusting it. */}
        <div className="mt-3 border-t border-line px-6 pb-[22px] pt-4">
          <form action="/auth/logout" method="post">
            <button
              type="submit"
              className="flex min-h-[44px] w-full items-center justify-center rounded-pill border border-line-strong text-[14.5px] font-semibold text-muted motion-press"
            >
              Sign out
            </button>
          </form>
          <p className="mt-2.5 text-[12.5px] leading-[1.5] text-faint">
            Signing out clears where you were on every tab. Sessions last weeks, so this is
            rarely worth doing by accident.
          </p>
        </div>
      </Modal>
    </>
  );
}
