import { useEffect } from "react";
import { Sidebar, SyndraRoot, UiViewProvider, useUiView } from "ui";

const frame = { height: 960, display: "flex", background: "var(--canvas)" } as const;

/** Pins the operator's view; the provider persists it, so every cell sets its own. */
function View({ view, children }: { view: "basic" | "advanced"; children: React.ReactNode }) {
  const { setView } = useUiView();
  useEffect(() => setView(view), [setView, view]);
  return <>{children}</>;
}

/** Navigation comes from `src/lib/nav.ts` (BASIC_NAV); the route marks the active row. */
export const OperatorBasic = () => (
  <SyndraRoot pathname="/users">
    <UiViewProvider isOperator>
      <View view="basic">
        <div style={frame}><Sidebar /></div>
      </View>
    </UiViewProvider>
  </SyndraRoot>
);

/** ADVANCED_NAV: Bundles, Automation and Review groups join the Basic set. */
export const OperatorAdvanced = () => (
  <SyndraRoot pathname="/governance/drift">
    <UiViewProvider isOperator>
      <View view="advanced">
        <div style={frame}><Sidebar /></div>
      </View>
    </UiViewProvider>
  </SyndraRoot>
);

export const Member = () => (
  <SyndraRoot pathname="/">
    <UiViewProvider isOperator={false}>
      <div style={frame}><Sidebar /></div>
    </UiViewProvider>
  </SyndraRoot>
);
