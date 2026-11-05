"use client";

import { Kbd } from "@/components/ui/Kbd";
import { Modal, ModalHeader } from "@/components/ui/Modal";
import { KEY_ROWS } from "@/lib/keys";

/**
 * The Keys sheet (A-19 P7): every key Syndra answers to, once, opened by `?`.
 * Plain words and drawn keys; no verb has a letter, so none is listed.
 */
export function KeysSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Modal open={open} onClose={onClose} size="sm">
      <ModalHeader title="Keys" lede="These work on every page. A key pressed in a field belongs to the field." />
      <dl data-keys-sheet className="flex flex-col px-6 pb-5">
        {KEY_ROWS.map(({ keys, does }) => (
          <div key={does} className="row-divider flex min-h-[var(--tap)] items-center gap-4 py-2">
            <dt className="flex w-24 flex-none items-center gap-1">
              {keys.map((key) => (
                <Kbd key={key}>{key}</Kbd>
              ))}
            </dt>
            <dd className="text-[14.5px] text-ink">{does}</dd>
          </div>
        ))}
      </dl>
    </Modal>
  );
}
