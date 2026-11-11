import { MySystems } from "@/components/member/MySystems";

/**
 * A member's own storage view. Present for every member whatever they can
 * reach — the page answers "can I get access?", which a missing nav row does
 * not.
 */
export default function StoragePage() {
  return <MySystems />;
}
