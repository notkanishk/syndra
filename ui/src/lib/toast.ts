// Thin semantic wrapper around `sonner`. The component-side dependency is
// just the toast() function; the <Toaster /> component is mounted once in
// the root layout. Keeping callers off the raw sonner API gives us a single
// place to swap libraries or tune defaults later.

import { toast } from "sonner";

export function toastSuccess(message: string, description?: string) {
  toast.success(message, { description });
}

export function toastError(message: string, description?: string) {
  toast.error(message, { description });
}

export function toastInfo(message: string, description?: string) {
  toast(message, { description });
}

export function toastPromise<T>(
  promise: Promise<T>,
  messages: { loading: string; success: string; error: string },
): Promise<T> {
  toast.promise(promise, messages);
  return promise;
}
