'use client';

import React, {useEffect, useState} from 'react';
// @ts-ignore
import SyntaxHighlighter from 'react-syntax-highlighter/dist/cjs/prism';
// @ts-ignore
import {oneDark, oneLight} from 'react-syntax-highlighter/dist/cjs/styles/prism';
import {useComputedColorScheme} from '@mantine/core';

interface CodeBlockProps {
    code: string;
    language: string;
}

const CodeBlock: React.FC<CodeBlockProps> = ({code, language}) => {
    const [theme, setTheme] = useState(oneDark);
    const colorScheme = useComputedColorScheme('dark', { getInitialValueInEffect: true });

    useEffect(() => {
        setTheme(colorScheme === 'dark' ? oneDark : oneLight);
    }, [colorScheme]);

    return (
        <SyntaxHighlighter
            language={language}
            style={theme}
            customStyle={{width: "100%"}}
            showLineNumbers={true}
            wrapLines={true}
        >
            {code.trim()}
        </SyntaxHighlighter>
    );
};

export {CodeBlock};
