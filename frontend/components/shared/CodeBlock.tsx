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
    /** The language configuration's syntax: its Prism grammar, or "" for plain text. */
    syntax: string;
    highlightLine?: number | null;
}

const CodeBlock = ({code, syntax, highlightLine}: CodeBlockProps): ReactNode => {
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
      lineProps={(lineNumber: number) => {
        const isHighlighted = highlightLine !== undefined && highlightLine !== null && lineNumber === highlightLine;
        return isHighlighted
          ? {
            style: {
              backgroundColor: 'rgba(255, 0, 0, 0.18)',
              display: 'block',
              borderLeft: '4px solid #fa5252',
              paddingLeft: '6px',
              marginLeft: '-10px',
            },
          }
          : {style: {display: 'block'}};
      }}
    >
      {code.trim()}
    </SyntaxHighlighter>
  );
};

export {CodeBlock};
