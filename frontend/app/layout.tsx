import {ColorSchemeScript, mantineHtmlProps} from "@mantine/core";
import {Inter} from "next/font/google";

import {Providers} from "./providers";

import type {Metadata, Viewport} from "next";
import type {ReactNode} from "react";
import "@mantine/core/styles.css";
import "@mantine/dropzone/styles.css";
import "@mantine/notifications/styles.css";
import "./globals.css";

const inter = Inter({subsets: ["latin"]});

export const metadata: Metadata = {
  icons: {
    shortcut: "/gate_logo.svg",
  },
};

export const viewport: Viewport = {
  minimumScale: 1,
  initialScale: 1,
  width: "device-width",
  userScalable: false,
};

const RootLayout = ({children}: { children: ReactNode }): ReactNode => {
  return (
    <html lang="ru" className={inter.className} {...mantineHtmlProps}>
      <head>
        <ColorSchemeScript defaultColorScheme="dark" />
      </head>
      <body suppressHydrationWarning>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
};

export default RootLayout;
