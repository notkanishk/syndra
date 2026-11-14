import { cookies } from 'next/headers';
import localFont from 'next/font/local';

import { AppShell } from '@/components/shell/AppShell';
import { Providers } from '@/components/providers';
import { getSession } from '@/lib/session';
import { fetchMakerspaceFormat } from '@/lib/zone';
import { effectiveLocale } from '@/lib/locale';
import { effectiveZone } from '@/lib/zone-effective';
import { THEME_SCRIPT } from '@/lib/theme-script';
import { VIEW_COOKIE, parseUiView } from '@/lib/ui-view-cookie';
import './globals.css';

// Display face — page titles, card titles, dialog titles. Optical sizing is
// exposed so a 42px greeting and a 20px card title are drawn as the same face
// rather than the same outline scaled twice.
const bricolage = localFont({
  src: './fonts/bricolage-latin.woff2',
  display: 'swap',
  // Variable font: weight 200-800 and the opsz axis are both in the file.
  weight: '200 800',
  variable: '--font-bricolage',
});

// Body face — every row, label and paragraph.
const figtree = localFont({
  src: './fonts/figtree-latin.woff2',
  display: 'swap',
  weight: '400 700',
  variable: '--font-figtree',
});

// Mono — role keys, grant ids, claim names, token payloads. Never the body
// face for any of those: they are things an operator pastes into a config
// file, and the shape of the glyphs is the difference between reading them
// and re-reading them.
const jetbrains = localFont({
  src: './fonts/jetbrains-latin.woff2',
  display: 'swap',
  weight: '400 500',
  variable: '--font-jetbrains',
});

export const metadata = {
  title: 'Syndra',
  description: 'Access management for the makerspace',
  // Named so a home-screen icon says "Syndra" rather than the page title of
  // whatever screen it was installed from.
  applicationName: 'Syndra',
  appleWebApp: { capable: true, title: 'Syndra', statusBarStyle: 'black-translucent' as const },
};

/**
 * `viewportFit: "cover"` is the line that makes the safe-area insets real.
 *
 * Without it `env(safe-area-inset-bottom)` resolves to zero on every device,
 * so the tab bar, the sheets and the nav sheet were all padding by nothing —
 * on a phone with a home indicator the bottom row of tabs sits underneath it,
 * and the thing an operator taps by accident is whatever the system does with
 * a swipe from that edge.
 *
 * `themeColor` follows the theme rather than being one colour: the browser
 * paints the status bar and the URL bar with it, and a dark chrome above a
 * light page reads as a rendering fault. The two values are the `--ground`
 * each theme declares.
 */
export const viewport = {
  viewportFit: 'cover' as const,
  themeColor: [
    { media: '(prefers-color-scheme: dark)', color: '#080906' },
    { media: '(prefers-color-scheme: light)', color: '#f4f1fb' },
  ],
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  // In parallel: the zone and locale read is bounded (3 s, over the 2 s /healthz may spend on its database ping) and must not add to the session read.
  const [session, read] = await Promise.all([getSession(), fetchMakerspaceFormat()]);
  // What this render really uses: after the (write-once) set, not what was read.
  const timeZone = effectiveZone(read.zone);
  const locale = effectiveLocale(read.locale);
  // The view is rendered by the server, so the first paint is already the right one.
  const initialView = parseUiView((await cookies()).get(VIEW_COOKIE)?.value);
  const fontVars = `${bricolage.variable} ${figtree.variable} ${jetbrains.variable}`;

  return (
    <html lang="en" className={fontVars} data-theme="dark" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_SCRIPT }} />
      </head>
      <body className="bg-ground text-ink antialiased">
        <Providers timeZone={timeZone} locale={locale} hasSession={Boolean(session)} isOperator={session?.role === "admin"}>
          {session ? <AppShell session={session} initialView={initialView}>{children}</AppShell> : children}
        </Providers>
      </body>
    </html>
  );
}
