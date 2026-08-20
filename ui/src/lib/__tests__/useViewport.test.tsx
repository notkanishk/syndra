// @vitest-environment jsdom
import { renderHook, act } from "@testing-library/react";
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
  it("answers false before the effect runs, so the server and the client agree", () => {
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
