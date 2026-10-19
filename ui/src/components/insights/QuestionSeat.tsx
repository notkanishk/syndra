"use client";

import { Fragment, useMemo, useRef, useState, type RefObject } from "react";

import { sentence, suggest, type Entity, type Part, type Question } from "@/components/insights/question";
import { AnchoredPanel } from "@/components/ui/Anchored";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";

// The spaces between parts are for readers of the text; the flex gap does the spacing.
const Parts = ({ parts }: { parts: Part[] }) => (
  <>
    {parts.map((p, i) => (
      <Fragment key={i}>
        {"word" in p ? (
          <span className="text-[14px] text-muted tablet:whitespace-nowrap">{p.word}</span>
        ) : (
          <span className="max-w-full rounded-pill bg-tint-2 px-3 py-[3px] text-[13.5px] font-semibold [overflow-wrap:anywhere] tablet:whitespace-nowrap">{p.token}</span>
        )}{" "}
      </Fragment>
    ))}
  </>
);

/**
 * The question seat: one field that holds the question as a sentence. Typing
 * names things (people, roles, bundles, projects) and the questions those
 * names can ask are offered below; the one chosen stays as chips with a way to
 * clear it. Nothing is guessed from meaning: a name that matches nothing
 * offers nothing.
 */
export function QuestionSeat({
  entities,
  question,
  node,
  person,
  text,
  onText,
  onAsk,
  onClear,
  inputRef,
}: {
  entities: Entity[];
  question: Question | null;
  node: (id: string) => string;
  person: (id: string) => string;
  text: string;
  onText: (text: string) => void;
  onAsk: (q: Question) => void;
  onClear: () => void;
  inputRef: RefObject<HTMLInputElement | null>;
}) {
  const seat = useRef<HTMLDivElement | null>(null);
  const [active, setActive] = useState(0);
  const options = useMemo(() => suggest(text, entities), [text, entities]);
  const at = Math.min(active, Math.max(options.length - 1, 0));

  if (question)
    return (
      <div
        role="group"
        aria-label="The question the map is answering"
        className="flex min-h-10 min-w-0 flex-[1_1_320px] flex-wrap items-center gap-2 rounded-inner border border-accent-line bg-surface-1 py-1 pl-3.5 pr-1.5"
      >
        <Parts parts={sentence(question, node, person)} />
        <Button variant="ghost" size="sm" aria-label="Clear the question" onClick={onClear} className="ml-auto">
          ×
        </Button>
      </div>
    );

  const ask = (q: Question) => {
    onAsk(q);
    onText("");
    setActive(0);
  };

  return (
    <div ref={seat} className="relative min-w-0 flex-[1_1_320px]">
      <Input
        ref={inputRef}
        value={text}
        onChange={(e) => {
          onText(e.target.value);
          setActive(0);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" && options[at]) ask(options[at]);
          else if (e.key === "ArrowDown" && options.length) {
            e.preventDefault();
            setActive((at + 1) % options.length);
          } else if (e.key === "ArrowUp" && options.length) {
            e.preventDefault();
            setActive((at - 1 + options.length) % options.length);
          }
        }}
        placeholder="Ask about a person, a door, a role, or find anything"
        aria-label="Ask the map"
        aria-expanded={options.length > 0}
        aria-controls="insights-questions"
        role="combobox"
        aria-autocomplete="list"
      />
      <AnchoredPanel
        anchor={seat}
        open={options.length > 0}
        width="anchor"
        expectHeight={360}
        className="popover-in overflow-hidden rounded-panel border border-line-strong bg-surface-2 shadow-popover"
      >
        <div role="listbox" id="insights-questions" aria-label="Questions" className="flex flex-col gap-0.5 p-2">
          {options.map((q, i) => (
            <button
              key={JSON.stringify(q)}
              type="button"
              role="option"
              aria-selected={i === at}
              onClick={() => ask(q)}
              onMouseEnter={() => setActive(i)}
              className={`motion-tint flex min-h-[44px] w-full flex-wrap items-center gap-[7px] rounded-inner px-3 py-2 text-left desktop:min-h-0 ${i === at ? "bg-[var(--hover)]" : ""}`}
            >
              <Parts parts={sentence(q, node, person)} />
            </button>
          ))}
        </div>
      </AnchoredPanel>
    </div>
  );
}
