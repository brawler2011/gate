import {Anchor, type AnchorProps} from '@mantine/core';
import {Link} from "react-router-dom";

import type {ReactNode} from "react";
import type {PropsWithChildren} from 'react';

type Props = PropsWithChildren<AnchorProps & { href: string }>;

export const LinkAnchor = ({href, ...props}: Props): ReactNode => {
  return <Anchor component={Link} to={href} {...props} />;
};
