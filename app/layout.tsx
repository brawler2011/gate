"use client";

import '@mantine/core/styles.css';
import '@mantine/dropzone/styles.css';
import './globals.css';
import React, {Suspense} from 'react';
import {ColorSchemeScript, mantineHtmlProps, MantineProvider} from '@mantine/core';
import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {Notifications} from "@mantine/notifications";
import {Inter} from "next/font/google"
import { theme } from '@/lib/theme/theme';
import ReactScan from '@/components/ReactScan';

const queryClient = new QueryClient();
const inter = Inter({subsets: ["latin"]})

export default function RootLayout({children}: { children: React.ReactNode }) {    
    return (
        <html lang="ru" className={inter.className} {...mantineHtmlProps}>
        <head>
            <ColorSchemeScript defaultColorScheme="dark"/>
            <link rel="shortcut icon" href="/gate_logo.svg"/>
            <meta
                name="viewport"
                content="minimum-scale=1, initial-scale=1, width=device-width, user-scalable=no"
            />  
        </head>
        <body suppressHydrationWarning>
        <ReactScan />
        <QueryClientProvider client={queryClient}>
            <MantineProvider theme={theme} defaultColorScheme="dark" withGlobalClasses>
                <Notifications/>
                <Suspense>
                    {children}
                </Suspense>
            </MantineProvider>
        </QueryClientProvider>
        </body>
        </html>
    );
}
