// @vitest-environment jsdom
import { render, renderHook, screen, act } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useIsPhone, useIsTouch } from "@/lib/useViewport";
import { setMediaQuery } from "@/test-utils/media";

const PHONE = "(max-width: 44.99rem)";
const TOUCH = "(max-width: 67.49rem)";

/**
 * The only viewport questions JavaScript is allowed to ask.
 *
 * Everything structural is CSS; these exist for the decisions CSS cannot make,
 * and each one has a real caller: whether to raise a keyboard on mount, over a
 * sentence that says what an action does to somebody's access.
 */
describe("the two breakpoints, as a question", () => {
  // Answered during the first render, not corrected afterwards. The hook used
  // to hold the answer in state and fix it from an effect, which is one frame
  // of the desktop answer — and `autoFocus` below cannot survive one frame.
  //
  // Recorded from inside the render rather than read off `result.current`:
  // `renderHook` wraps in `act`, so effects have already flushed by the time
  // the result is readable and the effect-based hook would pass this too. The
  // frame this test is about is not observable from outside the component —
  // which is precisely why the bug survived the branch.
  it("answers the live query on its first render", () => {
    setMediaQuery(TOUCH, true);
    const seen: boolean[] = [];
    function Probe() {
      seen.push(useIsTouch());
      return null;
    }
    render(<Probe />);

    expect(seen[0]).toBe(true);
  });

  it("answers false where nothing has been asked, so the server and the client agree", () => {
    const { result } = renderHook(() => useIsTouch());
    expect(result.current).toBe(false);
  });

  it("reads a phone as both phone and touch", () => {
    setMediaQuery(PHONE, true);
    setMediaQuery(TOUCH, true);
    expect(renderHook(() => useIsPhone()).result.current).toBe(true);
    expect(renderHook(() => useIsTouch()).result.current).toBe(true);
  });

  // A tablet is not a phone and IS a thumb. Collapsing the two is how a
  // tablet ends up with a phone's layout or a desktop's touch targets.
  it("reads a tablet as touch but not as a phone", () => {
    setMediaQuery(PHONE, false);
    setMediaQuery(TOUCH, true);
    expect(renderHook(() => useIsPhone()).result.current).toBe(false);
    expect(renderHook(() => useIsTouch()).result.current).toBe(true);
  });

  it("follows the viewport when it changes under a rotation", () => {
    setMediaQuery(TOUCH, false);
    const { result } = renderHook(() => useIsTouch());
    expect(result.current).toBe(false);

    act(() => setMediaQuery(TOUCH, true));
    expect(result.current).toBe(true);
  });
});

/**
 * The one caller that cannot survive a wrong first frame. React applies
 * `autoFocus` while it commits the node, so a hook that answers `false` first
 * and corrects itself afterwards has already opened the keyboard by the time
 * it tells the truth — and flipping the prop then does nothing.
 */
describe("a keyboard that does not open on a phone", () => {
  function KeyboardGate() {
    const touch = useIsTouch();
    return <input aria-label="name" autoFocus={!touch} />;
  }

  it("does not autofocus on a touch viewport", () => {
    setMediaQuery(TOUCH, true);
    render(<KeyboardGate />);

    expect(screen.getByLabelText("name")).not.toBe(document.activeElement);
  });

  it("still autofocuses where there is a pointer and no keyboard to open", () => {
    render(<KeyboardGate />);

    expect(screen.getByLabelText("name")).toBe(document.activeElement);
  });
});
