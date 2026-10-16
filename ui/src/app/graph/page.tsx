import { redirect } from "next/navigation";

/** The Access map became Insights (N12). The old address keeps working. */
export default function GraphRedirect() {
  redirect("/insights");
}
