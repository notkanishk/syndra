"use client";

import Link from "next/link";
import { useState } from "react";

import { ChimeToggle } from "@/components/settings/ChimeToggle";
import { Button } from "@/components/ui/Button";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import { SyndraMark, WELCOMED } from "@/components/shell/SyndraMark";
import { FoldChevron } from "@/components/ui/FoldChevron";
import { Segmented } from "@/components/ui/Select";
import { STORAGE_NOTE } from "@/components/kinds/storage/captions";
import { firstName } from "@/lib/greeting";
import { Unfold } from "@/lib/motion";
import { accountNav } from "@/lib/nav";
import { useMySystems } from "@/lib/queries/useMySystems";
import type { SessionUser } from "@/lib/session";
import { useTheme } from "@/lib/theme";
import { useInstallPrompt } from "@/lib/useInstallPrompt";

/** What each of an operator's own pages is for, under its link; a system's own page, the rest. */
const OWN_NOTES: Record<string, string> = {
  "/me": "What you can use, and asking for more",
  "/storage": STORAGE_NOTE,
};
const SYSTEM_NOTE = "Your account here, and what to do next";

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
  const [about, setAbout] = useState(false);
  const { choice, setTheme } = useTheme();
  const installer = useInstallPrompt();
  // An operator's own systems that are not storage, from the read a member's
  // rail draws from, so their own laser account has a way in.
  const operator = session.role !== "user";
  const own = accountNav(useMySystems(operator).data ?? []);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        aria-expanded={open}
        className="flex min-h-[var(--tap)] items-center gap-[9px] rounded-pill pl-1 pr-1 text-left control press-scale hover:bg-[var(--hover)] tablet:pr-3"
      >
        <span
          aria-hidden
          className="avatar-fill flex h-[34px] w-[34px] flex-none items-center justify-center rounded-pill text-[12.5px] font-semibold text-muted"
        >
          {session.avatar}
        </span>
        {/* Who you are, by the name RULES uses for you: the first one (A-20
            P15). It goes when the room is narrow, where the avatar and the
            mark are the seat. The sheet keeps the full name and the address. */}
        <span className="hidden max-w-[180px] truncate text-[13.5px] text-muted tablet:block">
          {firstName(session)}
        </span>
        <span className="sr-only">Account</span>
      </button>

      <Modal open={open} onClose={() => setOpen(false)} size="sm" labelledBy="account-sheet-title">
        <ModalHeader title="Account" titleId="account-sheet-title" />

        <div className="flex flex-col gap-3.5 px-6 pb-2">
          <div className="flex items-center gap-3">
            <span
              aria-hidden
              className="avatar-fill flex size-[var(--tap)] flex-none items-center justify-center rounded-pill text-[15px] font-semibold text-muted"
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
          {operator && (
            <div className="flex flex-col border-y border-line py-1.5">
              {own.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  onClick={() => setOpen(false)}
                  className="-mx-2 flex min-h-[var(--tap)] flex-col justify-center gap-0.5 rounded-inner px-2 py-1.5 hover-tint"
                >
                  <span className="text-[14.5px] font-semibold text-accent-text">
                    {item.label} <span aria-hidden>→</span>
                  </span>
                  <span className="text-[13px] text-muted">{OWN_NOTES[item.href] ?? SYSTEM_NOTE}</span>
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
              value={choice}
              onChange={setTheme}
              options={[
                { value: "dark", label: "Dark" },
                { value: "light", label: "Light" },
                { value: "device", label: "Device" },
              ]}
            />
          </div>

          {/* Another setting about this device: the chime is a browser
              preference, so it sits beside Appearance rather than among the
              org-wide automation policy it used to share a box with. */}
          {/* Drift is an operator's page, so a member is never offered its sound (A-20 P15). */}
          {session.role !== "user" && (
            <div className="flex flex-col gap-2.5 rounded-inner border border-line px-4 py-3 text-[14.5px]">
              <span className="flex flex-col gap-0.5">
                <span>Sound when drift is found</span>
                <span className="text-[13px] text-muted">In this browser only</span>
              </span>
              <ChimeToggle />
            </div>
          )}

          {/* The one home of the story of the name (A-18 H1): every person reaches
              this sheet at every width. Closed at rest, never pushed at anyone. */}
          <div className="rounded-inner border border-line">
            <button
              type="button"
              aria-expanded={about}
              onClick={() => setAbout((on) => !on)}
              className="flex min-h-[var(--tap)] w-full items-center gap-2.5 rounded-inner py-1.5 pl-4 pr-2 text-left text-[14.5px] hover-tint"
            >
              <SyndraMark size={16} />
              <span className="flex-1">About Syndra</span>
              <FoldChevron open={about} />
            </button>
            <Unfold open={about}>
              <div className="flex flex-col gap-2.5 pb-4 pl-[42px] pr-4 pt-1">
                <p className="text-[14.5px] leading-[1.55]">
                  Syndra is named for Syn. In the Norse stories she keeps the door of the hall, and
                  shuts it on those who should not enter. When a case is argued, she speaks for the
                  one who refuses it.
                </p>
                <p className="text-[14.5px] leading-[1.55] text-muted">
                  In the makerspace, Zitadel is the door. It checks that you are who you say you are.
                  Syndra keeps the list of what that lets you use, and the reason for every name on it.
                </p>
                <p className="text-[12.5px] text-faint">Built in the lab it runs. Powered by Zitadel.</p>
              </div>
            </Unfold>
          </div>

          {/* Offered here rather than as a banner on arrival. A first-load
              prompt interrupts somebody who came to read one thing and gets
              dismissed by reflex; this is where a member already is when they
              are thinking about the app rather than about a machine they want
              to book. Absent entirely when there is nothing to offer. */}
          {installer && (
            <Button size="sm" className="w-full" onClick={() => void installer.install()}>
              Install as an app
            </Button>
          )}
        </div>

        {/* Below a hairline and last, because it is the one thing here that
            ends the session rather than adjusting it. */}
        <div className="mt-3 border-t border-line px-6 pb-[22px] pt-4">
          <form
            action="/auth/logout"
            method="post"
            onSubmit={() => {
              // The next sign-in is a new first paint: the mark lights again.
              try {
                window.sessionStorage.removeItem(WELCOMED);
              } catch {}
            }}
          >
            <Button type="submit" variant="outline" className="w-full">
              Sign out
            </Button>
          </form>
          <p className="mt-2.5 text-[12.5px] leading-[1.5] text-faint">
            Signing out signs you out in every open tab.
          </p>
        </div>
      </Modal>
    </>
  );
}
