import { CopyButton } from "@/components/docs/copy-button";

export function CodeBlock({
  code,
  language,
}: {
  code: string;
  language: string;
}) {
  return (
    <div className="rounded-xl bg-sand-100 p-1">
      <div className="flex items-center justify-between py-0.5 pr-0.5 pl-3">
        <p className="font-mono text-xs text-sand-800">{language || "text"}</p>
        <CopyButton
          text={code}
          className="inline-flex size-8 items-center justify-center rounded-full text-sand-800 hover:bg-white hover:text-accent-500"
        />
      </div>
      <pre className="overflow-x-auto rounded-lg bg-white p-4 font-mono text-sm/6 text-base-800">
        <code>{code}</code>
      </pre>
    </div>
  );
}
