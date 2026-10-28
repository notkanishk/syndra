/**
 * One quoted CSV cell. A cell a spreadsheet would read as a formula (it starts
 * with `=`, `+`, `-`, `@`, a tab or a carriage return) is prefixed with `'`, so
 * a person named `=HYPERLINK(...)` is shown as text instead of run (OWASP, CSV
 * injection).
 */
export function csvCell(value: unknown): string {
  let text = String(value ?? "");
  if (/^[=+\-@\t\r]/.test(text)) text = `'${text}`;
  return `"${text.replace(/"/g, '""')}"`;
}

export function toCsv(rows: unknown[][]): string {
  return rows.map((cells) => cells.map(csvCell).join(",")).join("\n");
}
