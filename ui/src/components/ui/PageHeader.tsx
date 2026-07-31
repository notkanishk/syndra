/**
 * The page's own title block. The breadcrumb says where you are; this says
 * what you are looking at, with its actions on the right.
 */
export function PageHeader({
  eyebrow,
  title,
  meta,
  actions,
  className = "",
}: {
  /** Quiet line above the title — a parent project, a category. */
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  /** Metadata row beneath: email · title · team · id, dot-separated. */
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
