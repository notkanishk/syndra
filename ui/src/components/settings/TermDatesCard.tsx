"use client";

import { useRef, useState } from "react";

import { UserName } from "@/components/names/UserName";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FormPart } from "@/components/ui/FormPart";
import { FieldError, Input } from "@/components/ui/Input";
import { useLeavingRows } from "@/components/ui/LeavingRows";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { Unfold } from "@/components/ui/Unfold";
import { ApiError } from "@/lib/api-client";
import { formatShortDate } from "@/lib/format";
import { describeFailure, sentence } from "@/lib/outcome";
import { MAX_NAME, MAX_TERMS, shortDay, termFieldErrors, termStanding, validateTerms } from "@/lib/term-dates";
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
  const standing = termStanding(terms);

  return (
    <Card>
      <CardHeader
        title="Term dates"
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
              {standing[i] && <span className="text-faint">{` · ${standing[i]}`}</span>}
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
type Day = "start" | "end";

/** Where a server key lands: `terms[1].start` → that row's First day. Anything else is said once, at the top. */
const FIELD = /^terms\[(\d+)\]\.(name|start|end)$/;

/**
 * 2b (A-07 §1-3): one row per term, the field names said once above the rows.
 * Desktop: Name · First day · Last day · Remove. Phone: Name, then the two days
 * side by side, the error under the pair, Remove at the right.
 */
const ROW_GRID =
  "grid grid-cols-2 items-start gap-x-3 gap-y-2.5 tablet:grid-cols-[minmax(0,1fr)_9.5rem_9.5rem_auto] tablet:gap-x-3.5";
/** The visible field name on a phone, drawn by CSS so the words are in the DOM once, above the rows. */
const PHONE_NAME =
  "before:mb-[7px] before:block before:text-[12.5px] before:font-semibold before:text-faint before:content-[attr(data-label)] tablet:before:hidden";

function EditTermDates({ terms, onClose }: { terms: Term[]; onClose: () => void }) {
  const next = useRef(terms.length);
  const [rows, setRows] = useState<Row[]>(() => terms.map((t, key) => ({ ...t, key })));
  // Shown after a Save, and gone when the field is edited: an error about what
  // was typed stops being true the moment it changes.
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [failure, setFailure] = useState<{ text: string; reference?: string } | null>(null);
  const [added, setAdded] = useState<number[]>([]);
  const [saved, setSaved] = useState(false);
  const save = useSaveTerms();
  // A removed row fades where it was and its gap settles after; one added opens to its height.
  const drawn = useLeavingRows(rows, (r) => String(r.key));

  const edit = (i: number, patch: Partial<Term>) => {
    setRows((rs) => rs.map((r, n) => (n === i ? { ...r, ...patch } : r)));
    setErrors((es) => {
      const rest = { ...es };
      for (const f of Object.keys(patch)) delete rest[`terms[${i}].${f}`];
      return rest;
    });
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
      else {
        const ref = e instanceof ApiError ? e.details?.request_id : undefined;
        setFailure({ text: sentence(describeFailure(e)), reference: typeof ref === "string" ? ref : undefined });
      }
    }
  }

  const aboutRows = (key: string) => {
    const m = FIELD.exec(key);
    return Boolean(m && Number(m[1]) < rows.length);
  };
  const general = Object.entries(errors).filter(([k]) => !aboutRows(k));
  const atMost = rows.length >= MAX_TERMS;

  return (
    <Modal open onClose={onClose} busy={save.isPending} size="lg" labelledBy="edit-terms-title">
      <ModalHeader
        title="Edit term dates"
        titleId="edit-terms-title"
        lede="Each term runs from its first day to its last, both included."
      />
      <div className="flex flex-col gap-4 px-6">
        {saved ? (
          <ActionOutcome
            word="Saved"
            outcome={{
              kind: "applied",
              message: "Trends and new this term now use these dates.",
              detail: "Asking and giving use them from now on. Access already given keeps the end it was given.",
            }}
          />
        ) : (
          <>
            <Unfold open={general.length > 0} gap={16}>
              <Notice tone="danger" blocking title="That needs another look.">
                {general.map(([, m]) => m).join(" ")} Nothing was changed.
              </Notice>
            </Unfold>
            <Unfold open={Boolean(failure)} gap={16}>
              {failure && (
                <Notice tone="danger" blocking title="The term dates didn't save." reference={failure.reference}>
                  {failure.text} Nothing was changed.
                </Notice>
              )}
            </Unfold>
            <div>
              <div aria-hidden className={`${ROW_GRID} hidden pb-2 text-[12.5px] font-semibold text-faint tablet:grid`}>
                <span>Name</span>
                <span>First day</span>
                <span>Last day</span>
                <span className="invisible">Remove</span>
              </div>
              {/* Twelve is the most, and the list scrolls inside the sheet rather than growing it. */}
              <div className="tablet:max-h-[52dvh] tablet:overflow-y-auto">
                {drawn.rows.map((r) => {
                  // A row that is going has no place in the form any more: its index is -1 and it
                  // reads what it said (it is inert, so nothing in it can be edited).
                  const i = rows.findIndex((x) => x.key === r.key);
                  const who = r.name.trim() || `Term ${i + 1}`;
                  const wrong = (f: keyof Term) => errors[`terms[${i}].${f}`];
                  return (
                    <FormPart key={r.key} appear={added.includes(r.key)} {...drawn.props(String(r.key))}>
                    <div data-term-row className={`row-divider ${ROW_GRID} py-3.5`}>
                      <div data-label="Name" className={`col-span-2 tablet:col-span-1 ${PHONE_NAME}`}>
                        <Input
                          aria-label={`${who}, name`}
                          value={r.name}
                          maxLength={MAX_NAME + 20}
                          aria-invalid={Boolean(wrong("name"))}
                          className={wrong("name") ? "border-danger-line" : ""}
                          onChange={(e) => edit(i, { name: e.target.value })}
                        />
                      </div>
                      {(["start", "end"] as Day[]).map((f) => (
                        <div key={f} data-label={f === "start" ? "First day" : "Last day"} className={PHONE_NAME}>
                          <Input
                            aria-label={`${who}, ${f === "start" ? "first" : "last"} day`}
                            type="date"
                            value={r[f]}
                            aria-invalid={Boolean(wrong(f))}
                            className={wrong(f) ? "border-danger-line" : ""}
                            onChange={(e) => edit(i, { [f]: e.target.value })}
                          />
                        </div>
                      ))}
                      <div className="order-last col-span-2 justify-self-end tablet:order-none tablet:col-span-1 tablet:self-center">
                        <Button
                          size="sm"
                          aria-label={`Remove ${who}`}
                          disabled={rows.length === 1 || save.isPending}
                          onClick={() => {
                            setRows((rs) => rs.filter((x) => x.key !== r.key));
                            setErrors({});
                          }}
                        >
                          Remove
                        </Button>
                      </div>
                      {(["name", "start", "end"] as const).map((f) => (
                        <FieldError
                          key={f}
                          data-term-field={`terms[${i}].${f}`}
                          className="col-span-2 mt-0! tablet:col-span-full"
                        >
                          {wrong(f)}
                        </FieldError>
                      ))}
                    </div>
                    </FormPart>
                  );
                })}
              </div>
            </div>
            <div className="flex items-center gap-2.5">
              <Button
                variant="ghost"
                size="sm"
                disabled={atMost || save.isPending}
                onClick={() => {
                  const key = next.current++;
                  setRows((rs) => [...rs, { key, name: "", start: "", end: "" }]);
                  setAdded((a) => [...a, key]);
                  setErrors({});
                }}
              >
                Add a term
              </Button>
              {atMost && <span className="text-[13px] text-faint">Twelve is the most.</span>}
            </div>
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
            <Button disabled={save.isPending} onClick={onClose}>
              Cancel
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
