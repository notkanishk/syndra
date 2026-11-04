/**
 * The time of day, in the reader's own clock: "Morning" before 12, "Afternoon"
 * before 17, "Evening" after. One keeper greets an operator on Home and a
 * member on My access, so both ask this one function (A-18 §4 idea 2).
 */
export function greeting(now: Date): string {
  const hour = now.getHours();
  if (hour < 12) return "Morning";
  if (hour < 17) return "Afternoon";
  return "Evening";
}
