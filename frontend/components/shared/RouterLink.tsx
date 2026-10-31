import {forwardRef} from "react";
import {Link} from "react-router-dom";

import type {AnchorHTMLAttributes} from "react";

type Props = Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href"> & {
  href: string;
  scroll?: boolean;
  prefetch?: boolean;
  replace?: boolean;
};
const RouterLink = forwardRef<HTMLAnchorElement, Props>(
  ({href, scroll, prefetch: _prefetch, ...props}, ref) => (
    <Link
      to={href}
      preventScrollReset={scroll === false}
      ref={ref}
      {...props}
    />
  ),
);
RouterLink.displayName = "RouterLink";
export default RouterLink;
