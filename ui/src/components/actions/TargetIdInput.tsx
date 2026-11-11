"use client";

import { useEffect, useRef, useState } from "react";

import { SCAN_ARIA, SCAN_BUTTON, SCAN_DENIED, SCAN_LEDE, SCAN_STARTING } from "@/components/actions/copy";
import { Button } from "@/components/ui/Button";
import { FieldLabel, Input } from "@/components/ui/Input";

/**
 * An id on the thing itself, typed or scanned off its label. A USB or Bluetooth
 * scanner is a keyboard wedge: it types the id and presses Enter, which the
 * surrounding form already submits, so the field needs no code for it. Never a
 * picker: there are thousands. The camera is an extra, drawn only where the
 * browser has the Shape Detection API (`BarcodeDetector`, Chromium today) and a
 * camera to open; without it the typed field is the whole control.
 */

/** The part of the Shape Detection API used here (WICG), which TypeScript's DOM lib does not carry. */
interface Detector {
  detect(source: HTMLVideoElement): Promise<Array<{ rawValue: string }>>;
}
type DetectorClass = new () => Detector;

/** Whether this browser can read a label with its camera. Decided after mount, so the server and the first paint agree. */
export function useCanScan(): boolean {
  const [can, setCan] = useState(false);
  useEffect(() => {
    setCan("BarcodeDetector" in window && Boolean(navigator.mediaDevices?.getUserMedia));
  }, []);
  return can;
}

export function TargetIdInput({
  id,
  label,
  hint,
  value,
  onChange,
  onScan,
  disabled,
  autoFocus,
}: {
  id: string;
  label: string;
  hint: string;
  value: string;
  onChange: (value: string) => void;
  /** Open the camera; the field offers it only where the browser can. */
  onScan: () => void;
  disabled?: boolean;
  autoFocus?: boolean;
}) {
  const canScan = useCanScan();
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <div className="flex flex-wrap items-start gap-2.5">
        <div className="min-w-[200px] flex-1">
          <Input
            id={id}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            disabled={disabled}
            autoFocus={autoFocus}
            autoComplete="off"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            inputMode="text"
            enterKeyHint="go"
            aria-describedby={`${id}-hint`}
          />
        </div>
        {canScan && (
          <Button type="button" variant="outline" aria-label={SCAN_ARIA} disabled={disabled} onClick={onScan}>
            {SCAN_BUTTON}
          </Button>
        )}
      </div>
      <p id={`${id}-hint`} className="mt-[7px] text-[13px] text-faint">
        {hint}
      </p>
    </div>
  );
}

/**
 * A live camera that reads one label. It exists only while shown: the camera is
 * asked for when it mounts (the press that opened it), the first label read is
 * handed back, and the stream is stopped when it unmounts, however it closes.
 */
export function CameraScanner({ onScan, onClose, closeLabel }: { onScan: (value: string) => void; onClose: () => void; closeLabel: string }) {
  const video = useRef<HTMLVideoElement>(null);
  const [state, setState] = useState<"starting" | "live" | "denied">("starting");
  // The latest callback, so the stream is not restarted when a parent re-renders.
  const found = useRef(onScan);
  found.current = onScan;

  useEffect(() => {
    let stopped = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let stream: MediaStream | undefined;
    const stop = () => {
      stopped = true;
      clearTimeout(timer);
      stream?.getTracks().forEach((track) => track.stop());
    };
    (async () => {
      try {
        const Detector = (window as unknown as { BarcodeDetector: DetectorClass }).BarcodeDetector;
        const detector = new Detector();
        stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" } });
        if (stopped) return stop();
        const el = video.current;
        if (!el) return stop();
        el.srcObject = stream;
        await el.play();
        setState("live");
        const look = async () => {
          if (stopped) return;
          try {
            const [first] = await detector.detect(el);
            if (first?.rawValue && !stopped) {
              stop();
              found.current(first.rawValue);
              return;
            }
          } catch {
            // A frame that could not be read is not an answer; the next one may be.
          }
          timer = setTimeout(look, 250);
        };
        void look();
      } catch {
        stop();
        setState("denied");
      }
    })();
    return stop;
  }, []);

  return (
    <div className="flex flex-col gap-3">
      <p className="m-0 text-[14px] text-muted">{SCAN_LEDE}</p>
      {state === "denied" ? (
        <p role="alert" className="m-0 text-[13.5px] text-danger-text">
          {SCAN_DENIED}
        </p>
      ) : (
        <div className="relative overflow-hidden rounded-inner border border-line-strong bg-surface-1">
          <video ref={video} muted playsInline className="block aspect-[4/3] w-full object-cover" />
          {state === "starting" && (
            <p role="status" className="absolute inset-0 m-0 grid place-items-center text-[13.5px] text-muted">
              {SCAN_STARTING}
            </p>
          )}
        </div>
      )}
      <div>
        <Button type="button" variant="outline" onClick={onClose}>
          {closeLabel}
        </Button>
      </div>
    </div>
  );
}
