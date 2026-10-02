import { useEffect } from "react";
import { SyndraRoot, TouchNav, UiViewProvider, useUiView } from "ui";

/** Pins the operator's view; the provider persists it, so every cell sets its own. */
function View({ view, children }: { view: "basic" | "advanced"; children: React.ReactNode }) {
  const { setView } = useUiView();
  useEffect(() => setView(view), [setView, view]);
  return <>{children}</>;
}

/** Full-bleed phone screen (390x844 viewport): the bar sits at its bottom. Destinations come from src/lib/nav.ts. */
const phone = { margin: -24, height: "100dvh", display: "flex", flexDirection: "column", justifyContent: "flex-end", background: "var(--canvas)" } as const;

export const MemberTabs = () => (
  <SyndraRoot pathname="/">
    <UiViewProvider isOperator={false}>
      <div style={phone}><TouchNav /></div>
    </UiViewProvider>
  </SyndraRoot>
);

export const OperatorBasicTabs = () => (
  <SyndraRoot pathname="/users">
    <UiViewProvider isOperator>
      <View view="basic"><div style={phone}><TouchNav /></div></View>
    </UiViewProvider>
  </SyndraRoot>
);

/** Advanced has too many destinations for tabs, so it collapses to one go-to bar. */
export const OperatorAdvancedSheetBar = () => (
  <SyndraRoot pathname="/governance/drift">
    <UiViewProvider isOperator>
      <View view="advanced"><div style={phone}><TouchNav /></div></View>
    </UiViewProvider>
  </SyndraRoot>
);
