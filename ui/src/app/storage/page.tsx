import { StoragePage as MyStorage } from "@/components/kinds/storage/StoragePage";

/**
 * A member's own storage view: systems of kind storage only. Present for every
 * member whatever they can reach — the page answers "can I get access?", which
 * a missing nav row does not.
 */
export default function StoragePage() {
  return <MyStorage />;
}
