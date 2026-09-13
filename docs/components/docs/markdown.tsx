import Link from "next/link";
import type { ReactNode } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { CodeBlock } from "@/components/docs/code-block";
import { slugify } from "@/lib/docs";

function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return `${node}`;
  if (Array.isArray(node)) return node.map(textOf).join("");
  if (node && typeof node === "object" && "props" in node) {
    return textOf((node.props as { children?: ReactNode }).children);
  }
  return "";
}

export function Markdown({ children }: { children: string }) {
  return (
    <div className="prose">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h2: ({ children }) => (
            <h2 id={slugify(textOf(children))}>{children}</h2>
          ),
          h3: ({ children }) => (
            <h3 id={slugify(textOf(children))}>{children}</h3>
          ),
          a: ({ href = "", children }) =>
            href.startsWith("http") ? (
              <a href={href} target="_blank" rel="noopener noreferrer">
                {children}
              </a>
            ) : (
              <Link href={href}>{children}</Link>
            ),
          pre: ({ node }) => {
            const code = node?.children[0];
            if (code?.type !== "element") return null;
            const [className = ""] =
              (code.properties.className as string[] | undefined) ?? [];
            const text = code.children
              .map((child) => (child.type === "text" ? child.value : ""))
              .join("");
            return (
              <CodeBlock
                code={text.replace(/\n$/, "")}
                language={className.replace("language-", "")}
              />
            );
          },
          table: ({ children }) => (
            <div className="-mx-4 overflow-x-auto px-4 sm:mx-0 sm:px-0">
              <table>{children}</table>
            </div>
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
