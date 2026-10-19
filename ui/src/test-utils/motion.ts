/**
 * Tests run still by default: every exit phase and drag in the product treats
 * a test run as reduced motion, so "the dialog is gone after Close" is true on
 * the next line. A test of the motion itself opts in, with fake timers, and the
 * setup file switches it back off after each test.
 */
export function motionInTests(on = true): void {
  (window as { __motionInTests?: boolean }).__motionInTests = on;
}
