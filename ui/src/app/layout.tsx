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
