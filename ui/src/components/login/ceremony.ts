/**
 * The doorway's choreography.
 *
 * Three scenes, told through the arch rather than through banners: it draws
 * itself top-down on arrival, retracts and dissolves when the door opens, and
 * closes into a complete amber line when the provider refuses.
 *
 * Hand-rolled WAAPI rather than an animation library, because there isn't one
 * in this project and one screen does not justify adding one. That comes with
 * exactly one trap, and this module exists to hold it in one place:
 *
 *   A finished `fill: "both"` animation keeps winning over inline styles long
 *   after it has dropped out of `element.getAnimations()`. Resetting by
 *   querying the element therefore silently does nothing, and the failure
 *   state sticks forever. Every Animation created here is kept in `running`
 *   and cancelled by reference.
 *
 * The two values that trap is worst for — the arch's mask and its border
 * colour — are not animated at all. They hang off `[data-scene]` in
 * globals.css, so restoring them is React dropping an attribute.
 */

export type Scene = "idle" | "opening" | "unreachable";

export interface Door {
  /** Cancels whatever is running, restores the authored values, plays `scene`. */
  play(scene: Scene): void;
  /** Keyboard parity: hold the orb lit from below, where the button is. */
  lock(): void;
  unlock(): void;
  dispose(): void;
}

/** Default easing. `EXIT` is the one the arch itself moves on. */
const EASE = "cubic-bezier(.22,.61,.36,1)";
const EXIT = "cubic-bezier(.16,1,.3,1)";

/** The button's resting offset — it straddles the arch's baseline. */
const REST_Y = "translateY(calc(50% + 14px))";

/** The ring is lit to a distance of 420px, and dark beyond it. */
const REACH = 420;

/**
 * Where the orb's lit arc points, and how bright, for a cursor `dx, dy` away
 * from its centre.
 *
 * The gradient itself is authored on `.login-orb-lit` in globals.css and takes
 * its angle and its falloff from two custom properties. This returns those two
 * numbers and the two opacities — nothing here assembles a gradient, and no
 * colour value leaves the stylesheet.
 *
 * The `+ 180` is load-bearing: a CSS linear-gradient puts its first stop on
 * the side *opposite* the stated angle, so aiming the gradient away from the
 * cursor is what lands the opaque end toward it.
 */
export function ringLight(dx: number, dy: number) {
  const deg = (Math.atan2(dx, -dy) * 180) / Math.PI + 180;
  const near = Math.max(0, 1 - Math.hypot(dx, dy) / REACH);
  return {
    angle: `${deg.toFixed(1)}deg`,
    reach: `${(70 + near * 14).toFixed(0)}%`,
    ring: 0.4 + near * 0.6,
    bloom: near * near,
  };
}

/** Lit from below — where the button is — while the button holds focus. */
const FOCUS_ANGLE = "0deg";
const FOCUS_REACH = "80%";

type Frames = Keyframe[];
type Timing = { duration: number; delay?: number; easing?: string };

export function openDoor(root: HTMLElement): Door {
  const find = (name: string) => root.querySelector<HTMLElement>(`[data-door="${name}"]`);

  const orb = find("orb");
  const lit = find("lit");
  const bloom = find("bloom");

  const reduced =
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches === true;

  // The button's two fills and two shadows live in globals.css like every
  // other colour in this system; WAAPI needs them as literals, so they are
  // read from there rather than restated here.
  const style = getComputedStyle(root);
  const token = (name: string) => style.getPropertyValue(name).trim();
  const paint = {
    fill: token("--door-fill"),
    fillQuiet: token("--door-fill-quiet"),
    label: token("--on-violet"),
    labelQuiet: token("--door-label-quiet"),
    shadow: token("--door-shadow-rest"),
    shadowQuiet: token("--door-shadow-quiet"),
  };

  const running: Animation[] = [];

  function move(name: string, frames: Frames, timing: Timing) {
    const el = find(name);
    // Absent because the scene doesn't render it, or unsupported because this
    // is a server render or a very old engine. Either way: no animation, and
    // `set()` has already put the element where it belongs.
    if (!el || typeof el.animate !== "function") return;
    running.push(
      el.animate(frames, {
        fill: "both",
        easing: EASE,
        ...timing,
        // Reduced motion still gets the state change, just none of the travel.
        duration: reduced ? 0 : timing.duration,
        delay: reduced ? 0 : (timing.delay ?? 0),
      }),
    );
  }

  function set(name: string, props: Partial<CSSStyleDeclaration>) {
    const el = find(name);
    if (!el) return;
    Object.assign(el.style, props);
  }

  /**
   * Back to the authored composition. Cancel by reference first — see the
   * module note — then write the values those animations were filling.
   */
  function rest() {
    for (const animation of running) {
      try {
        animation.cancel();
      } catch {
        // Already finished or detached. Nothing to undo.
      }
    }
    running.length = 0;

    set("arch-clip", { height: "100%" });
    set("arch", { opacity: "1" });
    set("wash", { opacity: "1" });
    // "" rather than a literal: the blur is authored in the stylesheet and
    // only the brightness multiplier was ever inline.
    set("pool", { opacity: "1", filter: "" });
    set("pool-amber", { opacity: "0" });
    set("eyebrow", { opacity: "1", transform: "none" });
    set("orb", { opacity: "1", transform: "none" });
    set("bloom", { opacity: "0" });
    set("wordmark", { opacity: "1", transform: "none" });
    set("action", {
      opacity: "1",
      transform: REST_Y,
      background: paint.fill,
      color: paint.label,
      boxShadow: paint.shadow,
    });
    set("syn", { opacity: "1", transform: "none" });
    set("credit", { opacity: "1" });
    set("handoff", { opacity: "0", transform: "none" });
    set("error", { opacity: "0", transform: "none" });
  }

  /** The arch draws from the apex down, and everything else arrives under it. */
  function entrance() {
    move("pool", [{ opacity: 0 }, { opacity: 1 }], { duration: 1500, delay: 150 });
    move("arch-clip", [{ height: "0%" }, { height: "100%" }], {
      duration: 950,
      delay: 120,
      easing: EXIT,
    });
    move("wash", [{ opacity: 0 }, { opacity: 1 }], { duration: 900, delay: 560 });
    move("eyebrow", [{ opacity: 0, transform: "translateY(6px)" }, { opacity: 1, transform: "none" }], {
      duration: 700,
      delay: 300,
    });
    move("orb", [{ opacity: 0, transform: "scale(.6)" }, { opacity: 1, transform: "scale(1)" }], {
      duration: 800,
      delay: 640,
      easing: EXIT,
    });
    move("wordmark", [{ opacity: 0, transform: "translateY(12px)" }, { opacity: 1, transform: "none" }], {
      duration: 750,
      delay: 820,
    });
    move(
      "action",
      [
        { opacity: 0, transform: "translateY(calc(50% + 26px))" },
        { opacity: 1, transform: REST_Y },
      ],
      { duration: 750, delay: 1000 },
    );
    move("syn", [{ opacity: 0, transform: "translateY(8px)" }, { opacity: 1, transform: "none" }], {
      duration: 750,
      delay: 1180,
    });
    move("credit", [{ opacity: 0 }, { opacity: 1 }], { duration: 700, delay: 1360 });
  }

  /**
   * The door opens. This is cover for the redirect's latency, not a gate in
   * front of it — the navigation has already been issued by the time this runs.
   *
   * The arch retracts *and* fades: retracting alone leaves the uprights with
   * cut ends. The wash goes to exactly 0, because at any residual opacity it
   * keeps holding the arch's silhouette after the stroke is gone.
   */
  function opening() {
    move(
      "action",
      [
        { transform: `${REST_Y} scale(1)` },
        { transform: `${REST_Y} scale(.975)`, offset: 0.4 },
        { transform: `${REST_Y} scale(1)` },
      ],
      { duration: 320 },
    );
    move(
      "action",
      [
        { background: paint.fill, color: paint.label, boxShadow: paint.shadow },
        { background: paint.fillQuiet, color: paint.labelQuiet, boxShadow: paint.shadowQuiet },
      ],
      { duration: 700, delay: 200 },
    );
    move("arch-clip", [{ height: "100%" }, { height: "50%" }], {
      duration: 900,
      delay: 220,
      easing: EXIT,
    });
    move("arch", [{ opacity: 1 }, { opacity: 0 }], { duration: 950, delay: 220 });
    move("wash", [{ opacity: 1 }, { opacity: 0 }], { duration: 780, delay: 200 });
    move("pool", [{ filter: "blur(48px) brightness(1)" }, { filter: "blur(48px) brightness(1.75)" }], {
      duration: 1000,
      delay: 200,
    });
    move("bloom", [{ opacity: 0 }, { opacity: 1 }], { duration: 800, delay: 200 });
    move("syn", [{ opacity: 1 }, { opacity: 0 }], { duration: 400 });
    move("handoff", [{ opacity: 0, transform: "translateY(8px)" }, { opacity: 1, transform: "none" }], {
      duration: 700,
      delay: 400,
    });
  }

  /**
   * The door stays shut. The stroke's fade and its colour are CSS, keyed off
   * `[data-scene]` — this is only the light going out around them.
   */
  function unreachable() {
    move("pool", [{ opacity: 1 }, { opacity: 0.15 }], { duration: 800 });
    move("pool-amber", [{ opacity: 0 }, { opacity: 1 }], { duration: 800 });
    move("orb", [{ opacity: 1 }, { opacity: 0.35 }], { duration: 700 });
    move("syn", [{ opacity: 1 }, { opacity: 0 }], { duration: 400 });
    move("error", [{ opacity: 0, transform: "translateY(8px)" }, { opacity: 1, transform: "none" }], {
      duration: 700,
      delay: 350,
    });
  }

  let focusLock = false;

  function track(event: PointerEvent) {
    if (focusLock || !orb || !lit || !bloom) return;
    const box = orb.getBoundingClientRect();
    const light = ringLight(
      event.clientX - (box.left + box.width / 2),
      event.clientY - (box.top + box.height / 2),
    );
    lit.style.setProperty("--door-ring-angle", light.angle);
    lit.style.setProperty("--door-ring-reach", light.reach);
    lit.style.opacity = light.ring.toFixed(3);
    bloom.style.opacity = light.bloom.toFixed(3);
  }

  // A reduced-motion visitor keeps the authored static top-lit mask.
  if (!reduced) window.addEventListener("pointermove", track, { passive: true });

  return {
    play(scene) {
      rest();
      if (scene === "opening") opening();
      else if (scene === "unreachable") unreachable();
      else if (!reduced) entrance();
    },
    lock() {
      focusLock = true;
      if (!lit || !bloom) return;
      lit.style.setProperty("--door-ring-angle", FOCUS_ANGLE);
      lit.style.setProperty("--door-ring-reach", FOCUS_REACH);
      lit.style.opacity = "1";
      bloom.style.opacity = ".9";
    },
    unlock() {
      focusLock = false;
      if (!lit || !bloom) return;
      // Removing the two properties falls back to the authored top-lit mask,
      // rather than leaving the ring lit from below at a button that no longer
      // has focus. Safe *because* the gradient lives in the stylesheet — this
      // is the same clearing that deletes a value which only ever lived inline.
      lit.style.removeProperty("--door-ring-angle");
      lit.style.removeProperty("--door-ring-reach");
      lit.style.opacity = ".5";
      bloom.style.opacity = "0";
    },
    dispose() {
      window.removeEventListener("pointermove", track);
      for (const animation of running) {
        try {
          animation.cancel();
        } catch {
          // Already gone.
        }
      }
      running.length = 0;
    },
  };
}
