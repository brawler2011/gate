import {useQuery} from '@tanstack/react-query';
import {refractor, type Syntax} from 'refractor/core';

import type {Root, Element, Text} from 'hast';

/** Registers the grammar named syntax; false when refractor has no such grammar. */
const loadGrammar = async (syntax: string): Promise<boolean> => {
  if (refractor.registered(syntax)) {
    return true;
  }
  if (!/^[a-z0-9-]+$/.test(syntax)) {
    return false;
  }
  // Vite turns this into a chunk per grammar refractor ships, loaded the first
  // time a language configuration's syntax names it.
  const grammar = await import(`../../node_modules/refractor/lang/${syntax}.js`)
    .then((module: {default: Syntax}) => module.default, () => undefined);
  if (!grammar) {
    return false;
  }
  refractor.register(grammar);
  return true;
};

/**
 * The grammar to highlight a language configuration's sources with: its syntax
 * once the grammar has loaded, "text" (no highlighting) until then or when the
 * grammar is unknown. Both refractor and react-syntax-highlighter see it.
 */
export const useGrammar = (syntax: string | undefined): string => {
  const {data} = useQuery({
    queryKey: ['grammar', syntax],
    queryFn: () => loadGrammar(syntax ?? ''),
    enabled: !!syntax,
    staleTime: Infinity,
  });
  return data && syntax ? syntax : 'text';
};

type HastNode = Element | Text;

/**
 * Wrap identifiers in text with variable span
 */
const wrapIdentifiers = (text: string): string =>
  text.replace(/([a-zA-Z_][a-zA-Z0-9_]*)/g, '<span class="token variable">$1</span>');

/**
 * Convert hast AST to HTML string
 * @param isTopLevel - whether this is a top-level text node (not inside a token span)
 */
const hastToHtml = (node: HastNode | Root, isTopLevel: boolean = true): string => {
  if (node.type === 'text') {
    const escaped = escapeHtml((node as Text).value);
    // Only wrap identifiers in top-level text nodes
    if (isTopLevel) {
      return wrapIdentifiers(escaped);
    }
    return escaped;
  }

  if (node.type === 'root') {
    return (node as Root).children.map((child) => hastToHtml(child as HastNode, true)).join('');
  }

  if (node.type === 'element') {
    const element = node as Element;
    const tag = element.tagName;
    const className = element.properties?.className;
        
    let attrs = '';
    if (className && Array.isArray(className)) {
      attrs = ` class="${className.join(' ')}"`;
    }

    // Children inside a span are not top-level
    const children = element.children
      .map((child) => hastToHtml(child as HastNode, false))
      .join('');

    return `<${tag}${attrs}>${children}</${tag}>`;
  }

  return '';
};

/**
 * Escape HTML special characters
 */
const escapeHtml = (str: string): string => {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
};

/**
 * Highlight code with a registered grammar (see useGrammar) and return HTML;
 * any other grammar leaves the code as plain text.
 */
export const highlightCode = (code: string, grammar: string): string => {
  if (!refractor.registered(grammar)) {
    return escapeHtml(code);
  }
  return hastToHtml(refractor.highlight(code, grammar), true);
};
