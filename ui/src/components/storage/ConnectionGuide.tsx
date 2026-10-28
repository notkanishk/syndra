"use client";

import { useState } from "react";

import { type Device } from "@/components/storage/device";
import { Button } from "@/components/ui/Button";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { FieldLabel } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Segmented, Select } from "@/components/ui/Select";

/**
 * Step by step, from a member's own device (proposed/ConnectionGuide.md, 4b).
 *
 * Four steps, in the device's own words for its menus and buttons. Nothing
 * says SMB to the member except the address itself.
 */

const DEVICES: Array<{ value: Device; label: string }> = [
  { value: "mac", label: "Mac" },
  { value: "windows", label: "Windows" },
  { value: "linux", label: "Linux" },
  { value: "ios", label: "iPhone/iPad" },
  { value: "android", label: "Android" },
];

/** One folder the guide can take a member to. */
export interface GuideFolder {
  label: string;
  share: string;
  /** Inside the share; empty for its root. */
  folder: string;
  smb: string;
  unc: string;
}

interface Step {
  lead: string;
  keys?: string[];
  tail?: string;
  copy?: string;
  creds?: boolean;
  note?: string;
}

function stepsFor(device: Device, host: string, place: GuideFolder): Step[] {
  switch (device) {
    case "mac":
      return [
        { lead: "Open Finder, then choose Go › Connect to Server in the menu bar, or press", keys: ["⌘", "K"], tail: "." },
        { lead: "Paste this address and click Connect.", copy: place.smb },
        {
          lead: "When asked, choose Registered User and sign in.",
          creds: true,
          note: "On your own Mac, tick Remember this password in my keychain so you are not asked again.",
        },
        { lead: "The folder opens in Finder and stays in the sidebar under Locations. To come back to it later, press", keys: ["⌘", "K"], tail: " again." },
      ];
    case "windows":
      return [
        { lead: "Open File Explorer, or press", keys: ["Win", "E"], tail: "." },
        { lead: "Click the address bar at the top, paste this address and press Enter.", copy: place.unc },
        {
          lead: "When Windows asks for a user name and password, sign in. If it offers your usual account, choose More choices, then Use a different account.",
          creds: true,
          note: "On your own computer, tick Remember my credentials.",
        },
        { lead: "To keep it one click away, right-click This PC, choose Map network drive, and paste the same address." },
      ];
    case "linux":
      return [
        { lead: "Open Files and choose Other Locations at the bottom of the sidebar, or press", keys: ["Ctrl", "L"], tail: " to type an address." },
        { lead: "Paste this address into Connect to Server and click Connect.", copy: place.smb },
        { lead: "Choose Registered User and sign in. Leave Domain as it is.", creds: true },
        { lead: "The folder appears in the sidebar. Press", keys: ["Ctrl", "D"], tail: " to bookmark it." },
      ];
    case "ios":
      return [
        { lead: "Open the Files app, tap the ⋯ button at the top, then Connect to Server." },
        { lead: "Paste this address and tap Connect.", copy: place.smb },
        { lead: "Choose Registered User and sign in.", creds: true },
        { lead: "The folder stays under Shared in the Browse tab." },
      ];
    case "android":
      return [
        {
          lead: "Android has no built-in way to open a network folder, so install a file manager that can. In the Play Store, look for one that mentions SMB or LAN.",
        },
        {
          lead: "In that app, add a network or LAN location using SMB. Most ask for the server and the share separately.",
          copy: host,
          note: place.folder
            ? `Server above; share: ${place.share}. Then open ${place.folder.split("/").join(" › ")} inside it.`
            : `Server above; share: ${place.share}.`,
        },
        { lead: "Sign in with your account.", creds: true },
        { lead: "The folder stays in the app's list of locations." },
      ];
  }
}

function Keys({ keys }: { keys: string[] }) {
  return (
    <span className="inline-flex gap-1 align-[1px]">
      {keys.map((k) => (
        <kbd
          key={k}
          className="inline-flex h-[22px] min-w-[22px] items-center justify-center rounded-[6px] border border-b-2 border-line-strong bg-surface-1 px-1.5 font-mono text-[12.5px] text-ink"
        >
          {k}
        </kbd>
      ))}
    </span>
  );
}

export function ConnectionGuide({
  open,
  onClose,
  host,
  folders,
  username,
  system,
  defaultDevice,
}: {
  open: boolean;
  onClose: () => void;
  host: string;
  folders: GuideFolder[];
  username: string;
  system: string;
  defaultDevice: Device;
}) {
  const [device, setDevice] = useState<Device>(defaultDevice);
  const [chosen, setChosen] = useState(0);
  const place = folders[Math.min(chosen, folders.length - 1)];
  if (!place) return null;
  const steps = stepsFor(device, host, place);

  return (
    <Modal open={open} onClose={onClose} size="lg" labelledBy="guide-title">
      <ModalHeader
        title={`Connect to ${system}, step by step`}
        titleId="guide-title"
        lede="Four steps, done once per device and folder. After that it opens like any folder on your computer."
      />
      <div className="flex flex-col gap-5 px-6 pb-2">
        <div className="flex flex-wrap items-end gap-x-6 gap-y-3">
          <div className="flex min-w-0 flex-[1_1_200px] flex-col items-start gap-1.5">
            <FieldLabel htmlFor="guide-device">Your device</FieldLabel>
            <span className="hidden whitespace-nowrap tablet:flex">
              <Segmented label="Your device" value={device} onChange={setDevice} options={DEVICES} />
            </span>
            <span className="w-full min-w-[200px] tablet:hidden">
              <Select
                id="guide-device"
                aria-label="Your device"
                value={device}
                onChange={(e) => setDevice(e.target.value as Device)}
              >
                {DEVICES.map((d) => (
                  <option key={d.value} value={d.value}>
                    {d.label}
                  </option>
                ))}
              </Select>
            </span>
          </div>
          {folders.length > 1 && (
            <div className="flex min-w-[160px] flex-[0_1_220px] flex-col gap-1.5">
              <FieldLabel htmlFor="guide-folder">Folder</FieldLabel>
              <Select id="guide-folder" value={String(chosen)} onChange={(e) => setChosen(Number(e.target.value))}>
                {folders.map((f, i) => (
                  <option key={f.smb} value={i}>
                    {f.label}
                  </option>
                ))}
              </Select>
            </div>
          )}
        </div>

        <ol className="m-0 flex list-none flex-col gap-[18px] p-0">
          {steps.map((s, i) => (
            <li key={i} className="flex items-start gap-3.5">
              <span
                aria-hidden
                className="inline-flex h-[26px] w-[26px] flex-none items-center justify-center rounded-pill border border-line-strong text-[13px] font-semibold text-muted"
              >
                {i + 1}
              </span>
              <div className="flex min-w-0 flex-1 flex-col gap-2 pt-0.5">
                <span className="text-[15px] leading-[1.6] text-ink [text-wrap:pretty]">
                  {s.lead}
                  {s.keys && (
                    <>
                      {" "}
                      <Keys keys={s.keys} />
                      {s.tail}
                    </>
                  )}
                </span>
                {s.copy && (
                  <div className="max-w-[520px]">
                    <CopyableValue value={s.copy} label="Address to paste" />
                  </div>
                )}
                {s.creds && (
                  <div className="grid max-w-[520px] grid-cols-[auto_1fr] items-baseline gap-x-4 gap-y-1.5 rounded-inner border border-line bg-surface-0 px-3.5 py-3">
                    <span className="text-[13px] text-muted">Name</span>
                    <span className="min-w-0 [overflow-wrap:anywhere] font-mono text-[14px]">{username}</span>
                    <span className="text-[13px] text-muted">Password</span>
                    <span className="text-[14px]">Your storage password, not your Syndra or Google one</span>
                  </div>
                )}
                {s.note && <span className="text-[13.5px] leading-[1.5] text-muted [text-wrap:pretty]">{s.note}</span>}
              </div>
            </li>
          ))}
        </ol>

        <div className="flex flex-col gap-2 border-t border-line pt-4">
          <span className="type-label text-label">If it doesn&apos;t connect</span>
          <ul className="m-0 flex flex-col gap-1 pl-[18px] text-[14px] leading-[1.55] text-muted [text-wrap:pretty]">
            <li>Look at the status beside {system} on this page. If it says offline, wait for it to come back.</li>
            <li>If your sign-in is refused, check you used your storage password. You can change it at the end of this page.</li>
            <li>
              Still stuck? Ask makerspace staff, and mention your account name,{" "}
              <span className="[overflow-wrap:anywhere] font-mono text-ink">{username}</span>.
            </li>
          </ul>
        </div>
      </div>
      <ModalFooter>
        <Button variant="outline" onClick={onClose}>
          Close
        </Button>
      </ModalFooter>
    </Modal>
  );
}
