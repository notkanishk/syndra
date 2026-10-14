import { useEffect } from "react";
import { SyndraRoot, UiViewProvider, ViewSwitch, useUiView } from "ui";

/** Pins the operator's view; the provider persists it, so every cell sets its own. */
function View({ view, children }: { view: "basic" | "advanced"; children: React.ReactNode }) {
  const { setView } = useUiView();
  useEffect(() => setView(view), [setView, view]);
  return <>{children}</>;
}

/** Not rendered for members at all, so there is no member cell. */
export const Basic = () => (
  <SyndraRoot>
    <UiViewProvider isOperator>
      <View view="basic"><div style={{ display: "flex" }}><ViewSwitch /></div></View>
    </UiViewProvider>
  </SyndraRoot>
);

export const Advanced = () => (
  <SyndraRoot>
    <UiViewProvider isOperator>
      <View view="advanced"><div style={{ display: "flex" }}><ViewSwitch /></div></View>
    </UiViewProvider>
  </SyndraRoot>
);
