"use client";

import React from "react";

interface JsonViewProps {
  value: unknown;
  /** When provided, keys/values from `compareWith` that differ are highlighted. */
  compareWith?: unknown;
  className?: string;
}

/**
 * Pretty-printed JSON with a tiny purpose-built tokenizer so we don't pull in
 * a syntax-highlighting dependency. Strings are quoted, keys are colored
 * separately from values, primitives get their own tone. Optional `compareWith`
 * highlights differing values in amber for the side-by-side compare mode.
 */
export function JsonView({ value, compareWith, className = "" }: JsonViewProps) {
  const lines = renderLines(value, compareWith);
  return (
    <pre className={`m-0 whitespace-pre-wrap break-all text-xs leading-5 ${className}`}>
      <code>
        {lines.map((line, i) => (
          <div key={i}>{line}</div>
        ))}
      </code>
    </pre>
  );
}

function renderLines(value: unknown, compare?: unknown): React.ReactNode[] {
  const out: React.ReactNode[] = [];
  walk(value, compare, "", 0, out);
  return out;
}

function walk(
  value: unknown,
  compare: unknown,
  pathSuffix: string,
  depth: number,
  out: React.ReactNode[],
) {
  const indent = "  ".repeat(depth);
  if (value === null) {
    out.push(<span>{indent}<span className="text-amber-500">null</span>{pathSuffix}</span>);
    return;
  }
  if (typeof value === "string") {
    const differs = compare !== undefined && compare !== value;
    out.push(
      <span>
        {indent}
        <span className={differs ? "text-amber-500 underline decoration-dashed" : "text-emerald-500"}>
          {JSON.stringify(value)}
        </span>
        {pathSuffix}
      </span>,
    );
    return;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    const differs = compare !== undefined && compare !== value;
    out.push(
      <span>
        {indent}
        <span className={differs ? "text-amber-500 underline decoration-dashed" : "text-sky-500"}>
          {String(value)}
        </span>
        {pathSuffix}
      </span>,
    );
    return;
  }
  if (Array.isArray(value)) {
    if (value.length === 0) {
      out.push(<span>{indent}[]{pathSuffix}</span>);
      return;
    }
    out.push(<span>{indent}[</span>);
    value.forEach((item, idx) => {
      const compareItem = Array.isArray(compare) ? compare[idx] : undefined;
      walk(item, compareItem, idx === value.length - 1 ? "" : ",", depth + 1, out);
    });
    out.push(<span>{indent}]{pathSuffix}</span>);
    return;
  }
  if (typeof value === "object") {
    const obj = value as Record<string, unknown>;
    const keys = Object.keys(obj);
    if (keys.length === 0) {
      out.push(<span>{indent}{"{}"}{pathSuffix}</span>);
      return;
    }
    out.push(<span>{indent}{"{"}</span>);
    keys.forEach((key, i) => {
      const subIndent = "  ".repeat(depth + 1);
      const trailing = i === keys.length - 1 ? "" : ",";
      const subValue = obj[key];
      const compareValue = compare && typeof compare === "object" && compare !== null
        ? (compare as Record<string, unknown>)[key]
        : undefined;
      const inlinePrimitive =
        subValue === null ||
        typeof subValue === "string" ||
        typeof subValue === "number" ||
        typeof subValue === "boolean";

      if (inlinePrimitive) {
        const differs = compareValue !== undefined && compareValue !== subValue;
        const formatted =
          subValue === null
            ? <span className="text-amber-500">null</span>
            : typeof subValue === "string"
              ? <span className={differs ? "text-amber-500 underline decoration-dashed" : "text-emerald-500"}>{JSON.stringify(subValue)}</span>
              : <span className={differs ? "text-amber-500 underline decoration-dashed" : "text-sky-500"}>{String(subValue)}</span>;
        out.push(
          <span>
            {subIndent}
            <span className="text-primary">{JSON.stringify(key)}</span>
            <span className="text-muted">: </span>
            {formatted}
            {trailing}
          </span>,
        );
      } else {
        out.push(
          <span>
            {subIndent}
            <span className="text-primary">{JSON.stringify(key)}</span>
            <span className="text-muted">: </span>
          </span>,
        );
        walk(subValue, compareValue, trailing, depth + 1, out);
      }
    });
    out.push(<span>{indent}{"}"}{pathSuffix}</span>);
  }
}
