"use client";

import React, {type ReactNode} from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import "katex/dist/katex.min.css";

const renderSafeImage = (
  problemId?: string
): ((props: React.ImgHTMLAttributes<HTMLImageElement> & { node?: unknown }) => ReactNode) => {
  const SafeImage = ({
    node: _node,
    src,
    alt,
    ...props
  }: React.ImgHTMLAttributes<HTMLImageElement> & { node?: unknown }): ReactNode => {
    if (!src) {
      return null;
    }

    let width: string | undefined;
    let height: string | undefined;
    let isCentered = false;
    let cleanSrc = src;

    const hashIndex = src.indexOf("#");
    if (hashIndex !== -1) {
      cleanSrc = src.slice(0, hashIndex);
      const hashParts = src.slice(hashIndex + 1).split("#");

      for (const part of hashParts) {
        if (part === "center" || part === "middle") {
          isCentered = true;
        } else if (/^\d+(x\d*)?$/.test(part)) {
          if (part.includes("x")) {
            const [w, h] = part.split("x");
            if (w) {
              width = `${w}px`;
            }
            if (h) {
              height = `${h}px`;
            }
          } else {
            width = `${part}px`;
          }
        } else if (/^(\d+(x\d*)?)-center$/.test(part) || /^center-(\d+(x\d*)?)$/.test(part)) {
          isCentered = true;
          const num = part.replace(/-?center-?/, "");
          if (num.includes("x")) {
            const [w, h] = num.split("x");
            if (w) {
              width = `${w}px`;
            }
            if (h) {
              height = `${h}px`;
            }
          } else {
            width = `${num}px`;
          }
        }
      }
    }

    if (
      problemId &&
      !cleanSrc.startsWith("http://") &&
      !cleanSrc.startsWith("https://") &&
      !cleanSrc.startsWith("data:") &&
      !cleanSrc.startsWith("/api/")
    ) {
      const filename = cleanSrc.replace(/^\.\//, "").replace(/^media\//, "");
      cleanSrc = `/api/problems/${problemId}/media/${filename}`;
    }

    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={cleanSrc}
        alt={alt || ""}
        style={{
          maxWidth: "100%",
          width: width,
          height: height || "auto",
          display: isCentered ? "block" : undefined,
          margin: isCentered ? "0 auto" : undefined,
        }}
        {...props}
      />
    );
  };
  return SafeImage;
};

export interface ProblemStatementProps {
  value: string;
  problemId?: string;
  className?: string;
}

export const ProblemStatement = ({
  value,
  problemId,
  className = "content",
}: ProblemStatementProps): ReactNode => {
  return (
    <div className={className}>
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        components={{
          img: renderSafeImage(problemId),
        }}
      >
        {value}
      </ReactMarkdown>
    </div>
  );
};
