import type {ImgHTMLAttributes, ReactNode} from "react";

type Props = ImgHTMLAttributes<HTMLImageElement> & {
  priority?: boolean;
  unoptimized?: boolean;
  fill?: boolean;
};
const Image = ({
  priority,
  unoptimized: _unoptimized,
  fill,
  style,
  ...props
}: Props): ReactNode => (
  <img
    loading={priority ? "eager" : "lazy"}
    style={
      fill
        ? {
          position: "absolute",
          inset: 0,
          width: "100%",
          height: "100%",
          ...style,
        }
        : style
    }
    {...props}
  />
);
export default Image;
