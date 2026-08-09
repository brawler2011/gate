'use client';

import {useComputedColorScheme} from '@mantine/core';
import React, {useEffect, useState} from 'react';
import cpp from 'react-syntax-highlighter/dist/esm/languages/prism/cpp';
import go from 'react-syntax-highlighter/dist/esm/languages/prism/go';
import python from 'react-syntax-highlighter/dist/esm/languages/prism/python';
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism-light';
import {oneDark, oneLight} from 'react-syntax-highlighter/dist/esm/styles/prism';

SyntaxHighlighter.registerLanguage('cpp', cpp);
SyntaxHighlighter.registerLanguage('go', go);
SyntaxHighlighter.registerLanguage('python', python);

interface CodeBlockProps {
    code: string;
    language: string;
}

const CodeBlock: React.FC<CodeBlockProps> = ({code, language}) => {
  const [theme, setTheme] = useState(oneDark);
  const colorScheme = useComputedColorScheme('dark', {getInitialValueInEffect: true});

  useEffect(() => {
    setTheme(colorScheme === 'dark' ? oneDark : oneLight);
  }, [colorScheme]);

  return (
    <SyntaxHighlighter
      language={language}
      style={theme}
      customStyle={{width: "100%"}}
      showLineNumbers
      wrapLines
    >
      {code.trim()}
    </SyntaxHighlighter>
  );
};

export {CodeBlock};
