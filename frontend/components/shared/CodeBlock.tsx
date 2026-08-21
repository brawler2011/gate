"use client";

import {useComputedColorScheme} from '@mantine/core';
import React, {useEffect, useState} from 'react';
import cpp from 'react-syntax-highlighter/dist/esm/languages/prism/cpp';
import go from 'react-syntax-highlighter/dist/esm/languages/prism/go';
import python from 'react-syntax-highlighter/dist/esm/languages/prism/python';
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism-light';
import {oneDark, oneLight} from 'react-syntax-highlighter/dist/esm/styles/prism';

import type {ReactNode} from "react";

SyntaxHighlighter.registerLanguage('cpp', cpp);
SyntaxHighlighter.registerLanguage('go', go);
SyntaxHighlighter.registerLanguage('python', python);

interface CodeBlockProps {
    code: string;
    language: string;
    highlightLine?: number | null;
}

const CodeBlock = ({code, language, highlightLine}: CodeBlockProps): ReactNode => {
  const [theme, setTheme] = useState(oneDark);
  const colorScheme = useComputedColorScheme('dark', {getInitialValueInEffect: true});

  useEffect(() => {
    setTheme(colorScheme === 'dark' ? oneDark : oneLight);
  }, [colorScheme]);

  return (
    <SyntaxHighlighter
      language={language}
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
