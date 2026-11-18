"use client";

import {useComputedColorScheme} from '@mantine/core';
import React, {useEffect, useState} from 'react';
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism-light';
import oneDark from 'react-syntax-highlighter/dist/esm/styles/prism/one-dark';
import oneLight from 'react-syntax-highlighter/dist/esm/styles/prism/one-light';

import {useGrammar} from "@/lib/highlightCode";

import type {ReactNode} from "react";

interface CodeBlockProps {
    code: string;
    /** Prism grammar name; empty means plain text. */
    syntax: string;
}

const CodeBlock = ({code, syntax}: CodeBlockProps): ReactNode => {
  const grammar = useGrammar(syntax);
  const [theme, setTheme] = useState(oneDark);
  const colorScheme = useComputedColorScheme('dark', {getInitialValueInEffect: true});

  useEffect(() => {
    setTheme(colorScheme === 'dark' ? oneDark : oneLight);
  }, [colorScheme]);

  return (
    <SyntaxHighlighter
      language={grammar}
      style={theme}
      customStyle={{width: "100%", margin: 0, borderRadius: 'var(--mantine-radius-sm)'}}
      showLineNumbers
      wrapLines
    >
      {code.trim()}
    </SyntaxHighlighter>
  );
};

export {CodeBlock};
