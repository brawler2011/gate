import type {ReactNode} from "react";

type Props = {
  children: ReactNode;
};

const SlugRootLayout = ({children}: Props): ReactNode => {
  return children;
};

export default SlugRootLayout;
