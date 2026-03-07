"use client";
import { Anchor, type AnchorProps } from '@mantine/core';
import Link from 'next/link';

type Props = AnchorProps & { href: string };

export function LinkAnchor({ href, ...props }: Props) {
  return <Anchor component={Link} href={href} {...props} />;
}
