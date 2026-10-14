import { useEffect } from "react";
import { OfflineBanner } from "ui";

/**
 * The banner follows the browser's own online/offline events. The cell fires
 * `offline` after mount, the same event a dropped workshop wifi produces.
 */
export const Offline = () => {
  useEffect(() => {
    window.dispatchEvent(new Event("offline"));
  }, []);
  return <OfflineBanner />;
};
