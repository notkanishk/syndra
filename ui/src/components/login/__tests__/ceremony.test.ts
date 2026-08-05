// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { openDoor, ringLight } from "@/components/login/ceremony";

/** Every element the choreography reaches for, and nothing else. */
const HOOKS = [
  "arch-clip",
  "arch",
  "wash",
  "pool",
  "pool-amber",
  "eyebrow",
  "orb",
  "lit",
  "bloom",
  "wordmark",
  "action",
  "syn",
  "credit",
  "handoff",
  "error",
];

interface FakeAnimation {
  /** Which `[data-door]` element this was started on. */
  hook: string;
  frames: Keyframe[];
  cancelled: boolean;
  cancel(): void;
}

let created: FakeAnimation[] = [];
const noAnimate = Object.getOwnPropertyDescriptor(Element.prototype, "animate");

function stage(): HTMLElement {
  const root = document.createElement("main");
  root.innerHTML = HOOKS.map((name) => `<div data-door="${name}"></div>`).join("");
  document.body.append(root);
  return root;
}

beforeEach(() => {
  created = [];
  Element.prototype.animate = function animate(this: HTMLElement, frames: unknown) {
    const animation: FakeAnimation = {
      hook: this.dataset?.door ?? "",
      frames: (Array.isArray(frames) ? frames : []) as Keyframe[],
      cancelled: false,
      cancel() {
        this.cancelled = true;
      },
    };
    created.push(animation);
    return animation as unknown as Animation;
  };
});

afterEach(() => {
  if (noAnimate) Object.defineProperty(Element.prototype, "animate", noAnimate);
  else delete (Element.prototype as Partial<Element>).animate;
  document.body.innerHTML = "";
});

describe("ringLight", () => {
  // A CSS gradient's first stop sits on the side OPPOSITE the stated angle,
  // so the angle points away from the cursor and the opaque end lands on it.
  it("lights the side the cursor is on", () => {
    // linear-gradient(180deg) runs downward, so its opaque first stop is at
    // the top — which is where the cursor is.
    expect(ringLight(0, -100).angle).toBe("180.0deg"); // cursor above → lit above
    expect(ringLight(0, 100).angle).toBe("360.0deg"); // cursor below → lit below
    expect(ringLight(-100, 0).angle).toBe("90.0deg"); // cursor left  → lit left
    expect(ringLight(100, 0).angle).toBe("270.0deg"); // cursor right → lit right
  });

  it("brightens toward the orb and goes dark at 420px", () => {
    expect(ringLight(0, 0).ring).toBeCloseTo(1);
    expect(ringLight(0, 0).bloom).toBeCloseTo(1);
    expect(ringLight(0, 0).reach).toBe("84%");

    expect(ringLight(0, 420).ring).toBeCloseTo(0.4);
    expect(ringLight(0, 420).bloom).toBeCloseTo(0);
    expect(ringLight(0, 420).reach).toBe("70%");
  });

  // The gradient is authored on .login-orb-lit; this only feeds it two numbers.
  it("emits no colour of its own", () => {
    const light = ringLight(30, -40);
    expect(light.angle + light.reach).not.toMatch(/#|rgb|gradient/);
  });

  it("clamps rather than going negative beyond reach", () => {
    const far = ringLight(4000, 4000);
    expect(far.ring).toBeCloseTo(0.4);
    expect(far.bloom).toBe(0);
  });
});

describe("openDoor", () => {
  it("cancels by reference, not by asking the element", () => {
    // A finished fill:"both" animation drops out of getAnimations() while its
    // filled values keep winning over inline styles. Resetting by querying the
    // element is the bug that left the failure state stuck forever.
    const door = openDoor(stage());

    door.play("idle");
    const entrance = [...created];
    expect(entrance.length).toBeGreaterThan(0);

    door.play("unreachable");
    expect(entrance.every((animation) => animation.cancelled)).toBe(true);
  });

  it("writes the authored values back after a scene that moved them", () => {
    const root = stage();
    const door = openDoor(root);
    const at = (name: string) => root.querySelector<HTMLElement>(`[data-door="${name}"]`)!;

    door.play("opening"); // retracts the arch to half and fades the wash out
    door.play("idle");

    expect(at("arch-clip").style.height).toBe("100%");
    expect(Number(at("wash").style.opacity)).toBe(1);
    expect(Number(at("arch").style.opacity)).toBe(1);
    expect(Number(at("error").style.opacity)).toBe(0);
  });

  it("stops listening once disposed", () => {
    const root = stage();
    const door = openDoor(root);
    const lit = root.querySelector<HTMLElement>('[data-door="lit"]')!;

    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 10, clientY: 10 }));
    expect(lit.style.getPropertyValue("--door-ring-angle")).not.toBe("");

    door.dispose();
    lit.style.removeProperty("--door-ring-angle");
    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 40, clientY: 40 }));
    expect(lit.style.getPropertyValue("--door-ring-angle")).toBe("");
  });

  // The breathing pool and the animated grain are CSS animations on the pool's
  // transform and the grain's background-position. They are never cancelled, so
  // the choreography must never animate either property — a WAAPI animation on
  // the pool's transform would silently outrank the breath and freeze it.
  it("leaves the properties the CSS ambience owns alone", () => {
    const door = openDoor(stage());
    for (const scene of ["idle", "opening", "unreachable"] as const) door.play(scene);

    const pool = created.filter((animation) => animation.hook === "pool");
    expect(pool.length).toBeGreaterThan(0);
    for (const animation of pool) {
      for (const frame of animation.frames) {
        expect(frame.transform, "the pool's transform belongs to doorBreath").toBeUndefined();
      }
    }

    expect(created.some((animation) => animation.hook === "grain")).toBe(false);
  });

  it("holds the ring lit from below while the button has focus", () => {
    const root = stage();
    const door = openDoor(root);
    const lit = root.querySelector<HTMLElement>('[data-door="lit"]')!;

    door.lock();
    expect(lit.style.getPropertyValue("--door-ring-angle")).toBe("0deg");
    expect(Number(lit.style.opacity)).toBe(1);

    // A pointer moving under the lock must not steal the ring back.
    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 300, clientY: 0 }));
    expect(lit.style.getPropertyValue("--door-ring-angle")).toBe("0deg");
    expect(Number(lit.style.opacity)).toBe(1);

    // Releasing falls back to the authored top-lit mask rather than leaving the
    // ring pointing at a button that no longer has focus.
    door.unlock();
    expect(lit.style.getPropertyValue("--door-ring-angle")).toBe("");
    expect(lit.style.getPropertyValue("--door-ring-reach")).toBe("");
    expect(Number(lit.style.opacity)).toBe(0.5);
  });
});
