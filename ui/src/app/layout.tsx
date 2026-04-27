import { Fraunces, Inter } from 'next/font/google';

import Sidebar from '@/components/Sidebar';
import { Providers } from '@/components/providers';
import { getSession } from '@/lib/session';
import './globals.css';

// Body face — high-frequency reading. CSS var consumed via @theme in globals.css.
const inter = Inter({
  subsets: ['latin'],
  display: 'swap',
  variable: '--font-inter',
});

// Display face — h1 hero surfaces only (login, page titles). Variable axes
// kept conservative; `display: 'swap'` prevents FOUC on hero pages by allowing
// Inter to render first while Fraunces streams in.
const fraunces = Fraunces({
  subsets: ['latin'],
  display: 'swap',
  axes: ['SOFT', 'WONK', 'opsz'],
  variable: '--font-fraunces',
});

export const metadata = {
  title: 'MkAuth — Control Plane',
  description: 'Identity Orchestration for Makerspace Infrastructure',
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const session = await getSession();
  const fontVars = `${inter.variable} ${fraunces.variable}`;

  if (!session) {
    return (
      <html lang="en" className={fontVars}>
        <body className="bg-background text-on-surface antialiased">
          <div className="bg-blob-hero" aria-hidden />
          <Providers>{children}</Providers>
        </body>
      </html>
    );
  }

  return (
    <html lang="en" className={fontVars}>
      <body className="flex h-screen bg-background text-on-surface overflow-hidden antialiased">
        <div className="bg-blob-hero" aria-hidden />
        <Providers>
          <Sidebar session={session} />
          <main className="relative z-10 flex-1 overflow-y-auto p-8">{children}</main>
        </Providers>
      </body>
    </html>
  );
}
