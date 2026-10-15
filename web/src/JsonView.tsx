/** Render captured text as text nodes only; highlighting never interprets HTML. */
export function JsonView({ text, query }: { text: string; query: string }) {
  if (query.trim()) {
    const needle = query.toLocaleLowerCase();
    const lower = text.toLocaleLowerCase();
    const pieces = [];
    let from = 0;
    let at = lower.indexOf(needle);
    while (at !== -1) {
      pieces.push(text.slice(from, at), <mark key={at} className="rounded-sm bg-yellow-soft text-accent">{text.slice(at, at + needle.length)}</mark>);
      from = at + needle.length;
      at = lower.indexOf(needle, from);
    }
    pieces.push(text.slice(from));
    return <>{pieces}</>;
  }
  const tokens = text.split(/("(?:\\.|[^"\\])*"\s*:|"(?:\\.|[^"\\])*"|\b(?:true|false|null|-?\d+(?:\.\d+)?)\b)/g);
  return <>{tokens.map((token, index) => <span key={index} className={/^".*:\s*$/.test(token) ? "json-key" : token.startsWith('"') ? "json-string" : /^(true|false|null|-?\d)/.test(token) ? "json-number" : undefined}>{token}</span>)}</>;
}
