"use client";

import {AppShell, AppShellFooter, AppShellHeader, AppShellMain, AppShellProps, Box,} from "@mantine/core";
import React from "react";
import {Footer} from '@/components/shared/Footer';

type LayoutProps = {
    children: React.ReactNode;
    headerConfig?: AppShellProps["header"];
    footerConfig?: AppShellProps["footer"];
    asideConfig?: AppShellProps["aside"];
    navbarConfig?: AppShellProps["navbar"];
    stylesConfig?: AppShellProps["styles"];
    paddingConfig?: AppShellProps["padding"];
    header?: React.ReactNode;
};

const Layout = ({
                    children,
                    headerConfig = undefined, // Значения по умолчанию
                    footerConfig = {height: 0},
                    navbarConfig = undefined,
                    asideConfig = undefined,
                    stylesConfig = {
                        header: {
                            position: "static",
                        },
                        footer: {
                            position: "static",
                            bottom: "auto",
                            width: "100%",
                            zIndex: "auto",
                        },
                        main: {
                            paddingBottom: `var(--mantine-spacing-lg)`,
                        },
                    },
                    paddingConfig = "md",
                }: LayoutProps) => {
    return (
        <AppShell
            header={headerConfig}
            footer={footerConfig}
            aside={asideConfig}
            navbar={navbarConfig}
            styles={stylesConfig}
            padding={paddingConfig}
        >
            {children}
        </AppShell>
    );
};

const DefaultLayoutClient = ({children, header, ...props}: LayoutProps) => {
    return (
        <Layout {...props}>
            <AppShellHeader>
                {header}
            </AppShellHeader>
            <AppShellMain>
                <Box maw="1920px" mx="auto" w="100%">
                    {children}
                </Box>
            </AppShellMain>
            <AppShellFooter withBorder={false}>
                <Footer/>
            </AppShellFooter>
        </Layout>
    );
};

export {Layout, DefaultLayoutClient};