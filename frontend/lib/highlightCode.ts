import { refractor } from 'refractor/core';
import python from 'refractor/python';
import cpp from 'refractor/cpp';
import go from 'refractor/go';
import c from 'refractor/c';
import type { Root, Element, Text } from 'hast';

// Register languages
refractor.register(python);
refractor.register(cpp);
refractor.register(go);
refractor.register(c);

type HastNode = Element | Text;

/**
 * Wrap identifiers in text with variable span
 */
function wrapIdentifiers(text: string): string {
    // Match identifiers that are not Python keywords
    const pythonKeywords = new Set([
        'False', 'None', 'True', 'and', 'as', 'assert', 'async', 'await',
        'break', 'class', 'continue', 'def', 'del', 'elif', 'else', 'except',
        'finally', 'for', 'from', 'global', 'if', 'import', 'in', 'is',
        'lambda', 'nonlocal', 'not', 'or', 'pass', 'raise', 'return', 'try',
        'while', 'with', 'yield'
    ]);
    
    return text.replace(/([a-zA-Z_][a-zA-Z0-9_]*)/g, (match) => {
        if (pythonKeywords.has(match)) {
            return match; // Don't wrap keywords
        }
        return `<span class="token variable">${match}</span>`;
    });
}

/**
 * Convert hast AST to HTML string
 * @param isTopLevel - whether this is a top-level text node (not inside a token span)
 */
function hastToHtml(node: HastNode | Root, isTopLevel: boolean = true): string {
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
}

/**
 * Escape HTML special characters
 */
function escapeHtml(str: string): string {
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

/**
 * Map language names to refractor language identifiers
 */
function mapLanguage(language: string): string {
    const languageMap: Record<string, string> = {
        'python': 'python',
        'cpp': 'cpp',
        'c++': 'cpp',
        'golang': 'go',
        'go': 'go',
        'c': 'c',
    };
    return languageMap[language.toLowerCase()] || 'c';
}

/**
 * Highlight code using refractor and return HTML string
 */
export function highlightCode(code: string, language: string): string {
    try {
        const lang = mapLanguage(language);
        const tree = refractor.highlight(code, lang);
        return hastToHtml(tree, true);
    } catch {
        // Fallback: return escaped code if highlighting fails
        return escapeHtml(code);
    }
}

