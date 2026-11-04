/** A tab's name with its count: the one copy, shared by a bundle's page and a retired bundle's. */
export function TabLabel({ text, n }: { text: string; n: number }) {
  return (
    <>
      {text} <span className="text-faint">{n}</span>
    </>
  );
}
