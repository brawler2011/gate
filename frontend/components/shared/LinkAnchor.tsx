"use client";
import { Anchor, type AnchorProps } from '@mantine/core';
import Link from 'next/link';
import type { PropsWithChildren } from 'react';

type Props = PropsWithChildren<AnchorProps & { href: string }>;

export function LinkAnchor({ href, ...props }: Props) {
  return <Anchor component={Link} href={href} {...props} />;
}
