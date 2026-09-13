import type { ElementContent } from "hast";
import Link from "next/link";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { CodeBlock } from "@/components/docs/code-block";
import { slugify } from "@/lib/docs";

function textOf(nodes: ElementContent[]): string {
  return nodes
    .map((node) => {
      if (node.type === "text") return node.value;
      if (node.type === "element") return textOf(node.children);
      return "";
    })
    .join("");
}

export function Markdown({ children }: { children: string }) {
  return (
    <div className="prose">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h2: ({ node, children }) => (
            <h2 id={slugify(textOf(node?.children ?? []))}>{children}</h2>
          ),
          h3: ({ node, children }) => (
            <h3 id={slugify(textOf(node?.children ?? []))}>{children}</h3>
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
            return (
              <CodeBlock
                code={textOf(code.children).replace(/\n$/, "")}
                language={String(code.properties.className ?? "").replace(
                  "language-",
                  "",
                )}
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
