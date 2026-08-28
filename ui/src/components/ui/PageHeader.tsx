/**
 * The page's own title block. The breadcrumb says where you are; this says
 * what you are looking at, with its actions on the right.
 *
 * `lede` is the page's purpose, in one or two sentences: what it shows, when
 * you would come here, and — for a queue — what happens if you do nothing
 * (plain-language-copy §4). It used to be smuggled through `meta`, so a page
 * carried a count there, or a sentence, or nothing, and eleven pages had no
 * sentence saying what they were for. The two are separate props so that a
 * count never displaces the sentence, and so the guard can see which pages
 * still lack one.
 */
export function PageHeader({
  eyebrow,
  title,
  lede,
  meta,
  actions,
  className = "",
}: {
  /** Quiet line above the title — a parent project, a category. */
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  /** What this page is for. One or two sentences; every page has one. */
  lede?: React.ReactNode;
  /** Metadata row beneath: email · title · team · id, dot-separated. Never a sentence. */
  meta?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`flex flex-wrap items-end gap-4 ${className}`}>
      <div className="min-w-0 flex-1">
        {eyebrow && <div className="mb-1 text-[13.5px] text-faint">{eyebrow}</div>}
        <h1 className="type-page-title">{title}</h1>
        {meta && <div className="mt-1.5 text-[14.5px] text-muted">{meta}</div>}
        {lede && (
          <p className="mt-2 max-w-[72ch] text-[14.5px] leading-[1.6] text-muted">{lede}</p>
        )}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2.5">{actions}</div>}
    </div>
  );
}

/** Dot separator for a metadata row. */
export function MetaDot() {
  return <span aria-hidden className="inline-block h-[3px] w-[3px] rounded-pill bg-ink/30" />;
}

/** A metadata row: children separated by 3px dots. */
export function MetaRow({ children }: { children: React.ReactNode[] }) {
  const items = children.filter(Boolean);
  return (
    <span className="flex flex-wrap items-center gap-3 text-[14.5px] text-muted">
      {items.map((item, index) => (
        <span key={index} className="flex items-center gap-3">
          {index > 0 && <MetaDot />}
          {item}
        </span>
      ))}
    </span>
  );
}
