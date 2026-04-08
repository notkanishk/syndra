import Sidebar from '@/components/Sidebar';
import './globals.css';
import { Inter } from 'next/font/google';

import { getSession } from '@/lib/session';

const inter = Inter({ subsets: ['latin'] });

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

  if (!session) {
    return (
      <html lang="en">
        <body className={`${inter.className} bg-background text-foreground`}>
          {children}
        </body>
      </html>
    );
  }

  return (
    <html lang="en">
      <body className={`${inter.className} flex h-screen bg-background text-foreground overflow-hidden`}>
        <Sidebar session={session} />
        <main className="flex-1 overflow-y-auto p-8">
          {children}
        </main>
      </body>
    </html>
  );
}
