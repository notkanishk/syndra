// Split from ConnectionGuide so the page can pick a default device without loading the guide.
export type Device = "mac" | "windows" | "linux" | "ios" | "android";

/** The device a browser is on; Mac when it can't tell. */
export function deviceFrom(userAgent: string): Device {
  if (/iPhone|iPad/.test(userAgent)) return "ios";
  if (/Android/.test(userAgent)) return "android";
  if (/Windows/.test(userAgent)) return "windows";
  if (/Linux/.test(userAgent)) return "linux";
  return "mac";
}
