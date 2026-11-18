import {useQuery} from '@tanstack/react-query';
import {refractor, type Syntax} from 'refractor/core';

import type {Root, Element, Text} from 'hast';

const loadGrammar = async (syntax: string): Promise<boolean> => {
  if (refractor.registered(syntax)) {
    return true;
  }
  if (!/^[a-z0-9-]+$/.test(syntax)) {
    return false;
  }
  // Let failed imports reject so the query retries instead of caching plain text.
  const grammar = await import(`../../node_modules/refractor/lang/${syntax}.js`) as {default: Syntax};
  refractor.register(grammar.default);
  return true;
};

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

const wrapIdentifiers = (text: string): string =>
  text.replace(/([a-zA-Z_][a-zA-Z0-9_]*)/g, '<span class="token variable">$1</span>');

const hastToHtml = (node: HastNode | Root, isTopLevel: boolean = true): string => {
  if (node.type === 'text') {
    const escaped = escapeHtml((node as Text).value);
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

    const children = element.children
      .map((child) => hastToHtml(child as HastNode, false))
      .join('');

    return `<${tag}${attrs}>${children}</${tag}>`;
  }

  return '';
};

const escapeHtml = (str: string): string => {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
};

export const highlightCode = (code: string, grammar: string): string => {
  if (!refractor.registered(grammar)) {
    return escapeHtml(code);
  }
  return hastToHtml(refractor.highlight(code, grammar), true);
};
