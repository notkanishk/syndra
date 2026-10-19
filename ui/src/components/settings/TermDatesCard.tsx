"use client";

import { useRef, useState } from "react";

import { UserName } from "@/components/names/UserName";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { ApiError } from "@/lib/api-client";
import { formatShortDate } from "@/lib/format";
import { describeFailure, sentence } from "@/lib/outcome";
import { MAX_NAME, MAX_TERMS, shortDay, termFieldErrors, validateTerms } from "@/lib/term-dates";
import { useSaveTerms, useTermSetting, type Term } from "@/lib/queries/useTerms";

/**
 * The term dates, set once for the whole makerspace (owner, 19 Oct 2026).
 * Trends, "new this term", "to the end of next term" and Ask for access all
 * read them, which is why they are not under Connected systems. This card is
 * the one write place: anywhere else that shows a term says so, read-only,
 * with a link here.
 */
export function TermDatesCard() {
  const setting = useTermSetting();
  const [editing, setEditing] = useState(false);
  const terms = setting.data?.terms ?? [];
  const setBy = setting.data?.set_by;
  const setAt = setting.data?.set_at;

  return (
    <Card>
      <CardHeader
        title="Term dates"
        note="For the whole makerspace"
        action={
          setting.data && (
            <Button size="sm" onClick={() => setEditing(true)}>
              Edit term dates
            </Button>
          )
        }
      />
      <p className="row-divider max-w-[86ch] px-5 py-4 text-[14.5px] leading-[1.55] text-muted [text-wrap:pretty]">
        Trends, &quot;new this term&quot; and &quot;to the end of next term&quot; read these. Terms move each year, so
        check them when one ends.
      </p>
      <ListStates
        isLoading={setting.isLoading}
        error={setting.data ? null : setting.error}
        isEmpty={terms.length === 0}
        onRetry={() => setting.refetch()}
        errorTitle="Couldn't load the term dates."
        skeleton={<RowSkeleton rows={3} avatar={false} label="Loading term dates" />}
        empty={<EmptyState title="No term dates are set." guidance="Edit term dates to add the first one." />}
      >
        {terms.map((t, i) => (
          <div
            key={`${t.name}-${i}`}
            data-term-row
            className="row-divider flex flex-wrap items-baseline gap-x-[18px] gap-y-1 px-5 py-3.5"
          >
            <span className="w-[170px] shrink-0 text-[14px] text-muted">{t.name}</span>
            <span className="text-[15px] tabular-nums">
              {shortDay(t.start)} to {shortDay(t.end)}
            </span>
          </div>
        ))}
        <p className="row-divider px-5 py-3 text-[13px] text-faint">
          {setBy && setAt ? (
            <>
              Set by <UserName id={setBy} fallback="Someone" /> on {formatShortDate(setAt)}.
            </>
          ) : (
            "Never set. These are the usual dates until you change them."
          )}
        </p>
      </ListStates>
      {editing && <EditTermDates terms={terms} onClose={() => setEditing(false)} />}
    </Card>
  );
}

type Row = Term & { key: number };

/** Where a server key lands: `terms[1].start` → that row's Start. Anything else is said once, below the rows. */
const FIELD = /^terms\[(\d+)\]\.(name|start|end)$/;

function EditTermDates({ terms, onClose }: { terms: Term[]; onClose: () => void }) {
  const next = useRef(terms.length);
  const [rows, setRows] = useState<Row[]>(() => terms.map((t, key) => ({ ...t, key })));
  // Shown after a Save, and gone on the next edit: an error about what was
  // typed stops being true the moment it changes.
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [failure, setFailure] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const save = useSaveTerms();

  const edit = (key: number, patch: Partial<Term>) => {
    setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...patch } : r)));
    setErrors({});
    setFailure(null);
  };

  const strip = (r: Row): Term => ({ name: r.name.trim(), start: r.start, end: r.end });

  async function send() {
    setFailure(null);
    const body = rows.map(strip);
    const found = validateTerms(body);
    setErrors(found);
    if (Object.keys(found).length) return;
    try {
      await save.mutateAsync(body);
      setSaved(true);
    } catch (e) {
      const fields = e instanceof ApiError && e.code === "VALIDATION_FAILED" ? termFieldErrors(e.details) : {};
      if (Object.keys(fields).length) setErrors(fields);
      else setFailure(sentence(describeFailure(e)));
    }
  }

  const aboutRows = (key: string) => {
    const m = FIELD.exec(key);
    return Boolean(m && Number(m[1]) < rows.length);
  };
  const general = Object.entries(errors).filter(([k]) => !aboutRows(k));

  return (
    <Modal open onClose={onClose} busy={save.isPending} size="md" labelledBy="edit-terms-title">
      <ModalHeader
        title="Edit term dates"
        titleId="edit-terms-title"
        lede="Each term runs from its first day to its last, both included."
      />
      <div className="flex flex-col gap-4 px-6">
        {saved ? (
          <ActionOutcome
            word="Saved"
            outcome={{ kind: "applied", message: "Trends and new this term now use these dates." }}
          />
        ) : (
          <>
            {rows.map((r, i) => (
              <div key={r.key} className="flex flex-col gap-2.5 rounded-inner border border-line-strong p-3.5">
                <Field id={`term-${r.key}-name`} label="Name" error={errors[`terms[${i}].name`]} at={`terms[${i}].name`}>
                  <Input
                    id={`term-${r.key}-name`}
                    value={r.name}
                    maxLength={MAX_NAME + 20}
                    aria-invalid={Boolean(errors[`terms[${i}].name`])}
                    onChange={(e) => edit(r.key, { name: e.target.value })}
                  />
                </Field>
                <div className="grid gap-2.5 tablet:grid-cols-2">
                  {(["start", "end"] as const).map((f) => (
                    <Field
                      key={f}
                      id={`term-${r.key}-${f}`}
                      label={f === "start" ? "Start" : "End"}
                      error={errors[`terms[${i}].${f}`]}
                      at={`terms[${i}].${f}`}
                    >
                      <Input
                        id={`term-${r.key}-${f}`}
                        type="date"
                        value={r[f]}
                        aria-invalid={Boolean(errors[`terms[${i}].${f}`])}
                        onChange={(e) => edit(r.key, { [f]: e.target.value })}
                      />
                    </Field>
                  ))}
                </div>
                <div>
                  <Button
                    size="sm"
                    aria-label={`Remove ${r.name.trim() || "this term"}`}
                    disabled={rows.length === 1 || save.isPending}
                    onClick={() => {
                      setRows((rs) => rs.filter((x) => x.key !== r.key));
                      setErrors({});
                    }}
                  >
                    Remove
                  </Button>
                </div>
              </div>
            ))}
            <div>
              <Button
                variant="ghost"
                disabled={rows.length >= MAX_TERMS || save.isPending}
                onClick={() => {
                  setRows((rs) => [...rs, { key: next.current++, name: "", start: "", end: "" }]);
                  setErrors({});
                }}
              >
                Add a term
              </Button>
              {rows.length >= MAX_TERMS && (
                <span className="ml-2.5 text-[13px] text-faint">Twelve is the most.</span>
              )}
            </div>
            {general.length > 0 && (
              <Notice tone="danger" title="That needs another look.">
                {general.map(([, m]) => m).join(" ")} Nothing was changed.
              </Notice>
            )}
            {failure && (
              <Notice tone="danger" title="The term dates didn't save.">
                {failure} Nothing was changed.
              </Notice>
            )}
          </>
        )}
      </div>
      <ModalFooter
        note={
          saved ? undefined : "Saving changes the dates for everyone. Nothing about anyone's access changes."
        }
      >
        {saved ? (
          <Button onClick={onClose}>Close</Button>
        ) : (
          <>
            <Button variant="accent" isPending={save.isPending} disabled={save.isPending} onClick={send}>
              Save term dates
            </Button>
            <Button variant="ghost" disabled={save.isPending} onClick={onClose}>
              Cancel
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}

function Field({
  id,
  label,
  at,
  error,
  children,
}: {
  id: string;
  label: string;
  /** The key the server's 400 uses for this field. */
  at: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div data-term-field={at}>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {children}
      {error && (
        <p role="alert" className="mt-[7px] text-[13px] text-danger-text">
          {error}
        </p>
      )}
    </div>
  );
}
