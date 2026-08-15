"use client";

import {Anchor, type AnchorProps} from '@mantine/core';
import Link from 'next/link';

import type {ReactNode} from "react";
import type {PropsWithChildren} from 'react';

type Props = PropsWithChildren<AnchorProps & { href: string }>;

export const LinkAnchor = ({href, ...props}: Props): ReactNode => {
  return <Anchor component={Link} href={href} {...props} />;
};
