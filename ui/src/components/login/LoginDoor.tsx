"use client";

import { useEffect, useRef, useState } from "react";

import type { LoginFailure } from "@/lib/login-error";
import { openDoor, type Door, type Scene } from "./ceremony";

export interface DemoIdentity {
  id: string;
  name: string;
  role: "admin" | "user";
}

interface Props {
  /** `demo` is local development, where no identity provider is configured. */
  mode: "oidc" | "demo";
  /** Empty in `oidc` mode — a live deployment never serialises the catalog. */
  identities: DemoIdentity[];
  /** Present when the visitor arrived back here from a failed round trip. */
  failure: LoginFailure | null;
  /**
   * Where to return them once they are through the door — already validated
   * by `safeReturnPath`. Defaults to `/`: somebody who arrived here directly
   * has no destination, and the door must not invent one.
   */
  next?: string;
}

/**
 * The doorway. One arch, one orb, one action.
 *
 * There is no email field, no password field, no "or continue with", no
 * sign-up and no password reset: Zitadel is the sole identity provider and
 * owns all of that. What is left is a door, and whether it opens.
 */
export function LoginDoor({ mode, identities, failure, next = "/" }: Props) {
  const stage = useRef<HTMLElement>(null);
  const door = useRef<Door | null>(null);

  // The failure is the state the page arrives in, not something it transitions
  // to — so it is already in the server-rendered HTML and there is no frame
  // where the arch is open before it shuts.
  const [scene, setScene] = useState<Scene>(failure ? "unreachable" : "idle");
  const [handedOff, setHandedOff] = useState(false);

  useEffect(() => {
    if (!stage.current) return;
    const choreography = openDoor(stage.current);
    door.current = choreography;
    return () => {
      choreography.dispose();
      door.current = null;
    };
  }, []);

  useEffect(() => {
    door.current?.play(scene);
  }, [scene]);

  // The label lags the press by 200ms so the button's own dip reads first.
  useEffect(() => {
    if (mode !== "oidc" || scene !== "opening") return;
    const timer = setTimeout(() => setHandedOff(true), 200);
    return () => clearTimeout(timer);
  }, [mode, scene]);

  // The action moved out of the button and into the list, so the focus does too.
  useEffect(() => {
    if (mode !== "demo" || scene !== "opening") return;
    stage.current?.querySelector<HTMLElement>("[data-door-identity]")?.focus();
  }, [mode, scene]);

  function act(event: React.MouseEvent<HTMLElement>) {
    if (scene === "unreachable") {
      event.preventDefault();
      // Drop ?error= so a reload doesn't re-raise a refusal already moved past.
      window.history.replaceState(null, "", window.location.pathname);
      setScene("idle");
      return;
    }
    if (scene === "opening") {
      event.preventDefault();
      return;
    }
    // A modified click opens a new tab; this page stays as it was.
    if (mode === "oidc" && (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)) {
      return;
    }
    // Deliberately not prevented in `oidc` mode: /auth/zitadel is a real link,
    // and the browser leaves while the door is still opening. The animation is
    // cover for the redirect's latency, not a gate in front of it.
    setScene("opening");
  }

  const label =
    scene === "unreachable"
      ? "Try again"
      : mode === "demo"
        ? "Choose an identity"
        : handedOff
          ? "Opening…"
          : "Sign in with Zitadel";

  const action = (
    <>
      <span>{label}</span>
      <span className="login-action-arrow" aria-hidden="true">
        →
      </span>
    </>
  );

  const handlers = {
    className: "login-action",
    "data-door": "action",
    onClick: act,
    onFocus: () => door.current?.lock(),
    onBlur: () => door.current?.unlock(),
  };

  return (
    <main className="login-stage" data-scene={scene} ref={stage}>
      {/* Light, noise, and the arch are the room. None of it is content. */}
      <div className="login-floor" aria-hidden="true" />
      <div className="login-pool" data-door="pool" aria-hidden="true" />
      <div className="login-pool-amber" data-door="pool-amber" aria-hidden="true" />
      <div className="login-grain" aria-hidden="true" />

      <p className="login-eyebrow" data-door="eyebrow">
        Makerspace Syndra
      </p>

      <div className="login-group">
        <span className="login-arch-clip" data-door="arch-clip" aria-hidden="true">
          <span className="login-arch" data-door="arch" />
        </span>
        <span className="login-wash" data-door="wash" aria-hidden="true" />

        <span className="login-orb-slot" aria-hidden="true">
          <span className="login-orb" data-door="orb">
            <span className="login-orb-base" />
            <span className="login-orb-lit" data-door="lit" />
            <span className="login-orb-bloom" data-door="bloom" />
            <span className="login-orb-dot" />
          </span>
        </span>

        <h1 className="login-wordmark" data-door="wordmark">
          Syndra
        </h1>

        {mode === "oidc" ? (
          <a href={next === "/" ? "/auth/zitadel" : `/auth/zitadel?next=${encodeURIComponent(next)}`} {...handlers}>
            {action}
          </a>
        ) : (
          <button type="button" {...handlers}>
            {action}
          </button>
        )}
      </div>

      <div className="login-base">
        {/* The gloss earns its place because "keeps the door" is literally the
            screen you are looking at. What follows it has to be Syndra's job,
            not more of Syn's biography — the voice is a doorkeeper, not a
            character, and a grant here always carries the reason it exists. */}
        <p className="login-syn" data-door="syn">
          Syn — the goddess who keeps the door. Syndra keeps the list, and the reason for every
          name on it.
        </p>
        <p className="login-credit" data-door="credit">
          <span>Built in the lab it runs.</span>
          <span className="login-credit-dot" aria-hidden="true" />
          <span>Powered by Zitadel</span>
        </p>
      </div>

      {/* Both slots stay mounted so their live regions exist before the change
          that fills them, and so the choreography always has a target. Their
          contents do not: opacity is not hidden, and a screen reader would
          otherwise read a message the page is not showing. */}
      <div className="login-message" data-door="handoff" role="status">
        {scene === "opening" &&
          (mode === "oidc" ? (
            <>
              <p className="login-message-head">Handing you to Zitadel.</p>
              <p className="login-message-sub">You&rsquo;ll come back here signed in.</p>
            </>
          ) : (
            <>
              <p className="login-message-head">Development identities</p>
              <div className="login-identities">
                {identities.map((identity) => (
                  <form key={identity.id} action="/auth/login" method="post">
                    <input type="hidden" name="userId" value={identity.id} />
                    <input type="hidden" name="next" value={next} />
                    <button type="submit" className="login-identity" data-door-identity>
                      {identity.name}{" "}
                      <span className="login-identity-role">
                        · {identity.role === "admin" ? "Operator" : "Member"}
                      </span>
                    </button>
                  </form>
                ))}
              </div>
            </>
          ))}
      </div>

      <div className="login-message login-error" data-door="error" role="alert">
        {scene === "unreachable" && failure && (
          <>
            <p className="login-message-head">{failure.head}</p>
            <p className="login-message-sub">{failure.sub}</p>
          </>
        )}
      </div>
    </main>
  );
}
