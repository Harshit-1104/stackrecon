import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { LearnListFAB } from "@/components/LearnListFAB";

const inter = Inter({ subsets: ["latin"], variable: "--font-sans" });

export const metadata: Metadata = {
  title: "StackRecon",
  description: "Your Stack. Their Signal. The Gap.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${inter.variable} font-sans antialiased`}>
        <main className="min-h-screen flex flex-col">
          {children}
        </main>
        <LearnListFAB />
      </body>
    </html>
  );
}
