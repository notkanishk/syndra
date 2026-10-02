import { useEffect } from "react";
import { SyndraRoot, TopBar, UiViewProvider, useUiView } from "ui";

const operator = {
  id: "284019337710552064",
  name: "Priya Nair",
  email: "priya.nair@ashoka.edu.in",
  title: "Makerspace Manager",
  team: "Makerspace",
  status: "active",
  avatar: "PN",
  role: "admin" as const,
  sessionType: "oidc" as const,
};

const member = {
  id: "284019337710552099",
  name: "Kabir Mehta",
  email: "kabir.mehta@ashoka.edu.in",
  title: "Student",
  team: "Design Club",
  status: "active",
  avatar: "KM",
  role: "user" as const,
  sessionType: "oidc" as const,
};

const frame = { background: "var(--canvas)" } as const;

/** Pins the operator's view; the provider persists it, so every cell sets its own. */
function View({ view, children }: { view: "basic" | "advanced"; children: React.ReactNode }) {
  const { setView } = useUiView();
  useEffect(() => setView(view), [setView, view]);
  return <>{children}</>;
}

/** Breadcrumb comes from `crumbsFor` in src/lib/nav.ts; the switch sits beside the account chip. */
export const OperatorBasic = () => (
  <SyndraRoot pathname="/users">
    <UiViewProvider isOperator>
      <View view="basic">
        <div style={frame}><TopBar session={operator} /></div>
      </View>
    </UiViewProvider>
  </SyndraRoot>
);

/** A grouped route: the group name leads the trail. */
export const OperatorAdvanced = () => (
  <SyndraRoot pathname="/governance/drift">
    <UiViewProvider isOperator>
      <View view="advanced">
        <div style={frame}><TopBar session={operator} /></div>
      </View>
    </UiViewProvider>
  </SyndraRoot>
);

/** Members get no view switch at all - not greyed, not present. */
export const Member = () => (
  <SyndraRoot pathname="/requests">
    <UiViewProvider isOperator={false}>
      <div style={frame}><TopBar session={member} /></div>
    </UiViewProvider>
  </SyndraRoot>
);
