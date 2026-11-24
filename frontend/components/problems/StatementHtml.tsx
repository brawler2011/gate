import katex from "katex";
import {useLayoutEffect, useRef, type ReactNode} from "react";

import "katex/dist/katex.min.css";

export type AssetUrl = (name: string) => string | undefined;

const entities: Record<string, string> = {"&amp;": "&", "&lt;": "<", "&gt;": ">", "&#34;": "\"", "&#39;": "'", "&quot;": "\""};

const assetName = (src: string): string => {
  const name = src.replace(/&(amp|lt|gt|quot|#34|#39);/g, (entity) => entities[entity]);
  try {
    return decodeURIComponent(name);
  } catch {
    return name;
  }
};

const attribute = (value: string): string => value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll("\"", "&#34;");

// graphicx tries these extensions after the name as given (statement.ImageFile).
const extensions = ["", ".pdf", ".png", ".jpg", ".jpeg"];

// The backend sanitized the HTML and writes src first; images outside the problem's assets show their alt text,
// and browsers show PDF images only as links.
const withAssets = (html: string, assetUrl?: AssetUrl): string =>
  html.replace(/<img src="([^"]*)"([^>]*)>/g, (_, src: string, rest: string) => {
    const name = assetName(src);
    for (const ext of extensions) {
      const url = assetUrl?.(name + ext);
      if (url && (name + ext).toLowerCase().endsWith(".pdf")) {
        return `<a href="${attribute(url)}" target="_blank" rel="noreferrer">${attribute(name + ext)}</a>`;
      }
      if (url) {
        return `<img src="${attribute(url)}"${rest}>`;
      }
    }
    return `<img${rest}>`;
  });

const delimiters = /^\\[([]|\\[)\]]$/g;

export const StatementHtml = ({html, assetUrl}: {html: string; assetUrl?: AssetUrl}): ReactNode => {
  const ref = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    ref.current?.querySelectorAll("span.math").forEach((element) => {
      // renderToString, unlike render, also works in quirks-mode documents.
      element.innerHTML = katex.renderToString((element.textContent ?? "").replace(delimiters, ""), {
        throwOnError: false,
        displayMode: element.classList.contains("display"),
      });
    });
  }, [html]);
  return <div className="content" ref={ref} dangerouslySetInnerHTML={{__html: withAssets(html, assetUrl)}} />;
};
