import { Bricolage_Grotesque, Figtree, JetBrains_Mono } from 'next/font/google';

import { AppShell } from '@/components/shell/AppShell';
import { Providers } from '@/components/providers';
import { getSession } from '@/lib/session';
import './globals.css';

// Display face — page titles, card titles, dialog titles. Optical sizing is
// exposed so a 42px greeting and a 20px card title are drawn as the same face
// rather than the same outline scaled twice.
const bricolage = Bricolage_Grotesque({
  subsets: ['latin'],
  display: 'swap',
  // Variable font: the weight axis comes for free, opsz is opted into.
  axes: ['opsz'],
  variable: '--font-bricolage',
});

// Body face — every row, label and paragraph.
const figtree = Figtree({
  subsets: ['latin'],
  display: 'swap',
  weight: ['400', '500', '600', '700'],
  variable: '--font-figtree',
});

// Mono — role keys, grant ids, claim names, token payloads. Never the body
// face for any of those: they are things an operator pastes into a config
// file, and the shape of the glyphs is the difference between reading them
// and re-reading them.
const jetbrains = JetBrains_Mono({
  subsets: ['latin'],
  display: 'swap',
  weight: ['400', '500'],
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
  const session = await getSession();
  const fontVars = `${bricolage.variable} ${figtree.variable} ${jetbrains.variable}`;

  // Dark is the default; the stored preference is applied before paint so a
  // light-theme operator never sees a frame of the dark room.
  const themeScript = `try{var t=localStorage.getItem('syndra-theme');document.documentElement.setAttribute('data-theme',t==='light'?'light':'dark')}catch(e){document.documentElement.setAttribute('data-theme','dark')}`;

  return (
    <html lang="en" className={fontVars} data-theme="dark" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeScript }} />
      </head>
      <body className="bg-ground text-ink antialiased">
        <Providers hasSession={Boolean(session)}>
          {session ? <AppShell session={session}>{children}</AppShell> : children}
        </Providers>
      </body>
    </html>
  );
}
