<script>
  import { renderMarkdown } from '../../utils/markdown.js';

  let {
    content = '',
    class: className = '',
  } = $props();

  let renderedHtml = $derived(renderMarkdown(content));
</script>

<div class="ai-markdown-container overflow-x-auto {className}">
  <div class="ai-markdown prose prose-sm md:prose-base prose-invert max-w-none text-slate-200">
    {@html renderedHtml}
  </div>
</div>

<style>
  /* Markdown table styling */
  :global(.ai-markdown table) {
    width: 100%;
    margin-top: 1rem;
    margin-bottom: 1rem;
    border-collapse: collapse;
    font-size: 0.8125rem; /* 13px */
    line-height: 1.5;
    border-radius: 0.5rem;
    overflow: hidden;
    box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.2);
  }

  :global(.ai-markdown th) {
    background-color: #1e293b; /* slate-800 */
    color: #f8fafc; /* slate-50 */
    font-weight: 600;
    text-align: left;
    padding: 0.625rem 0.875rem;
    border: 1px solid #334155; /* slate-700 */
    white-space: nowrap;
  }

  :global(.ai-markdown td) {
    padding: 0.625rem 0.875rem;
    border: 1px solid #334155; /* slate-700 */
    vertical-align: top;
    word-break: break-word;
    color: #cbd5e1; /* slate-300 */
  }

  :global(.ai-markdown tr:nth-child(even)) {
    background-color: rgba(30, 41, 59, 0.4); /* subtle slate-800 */
  }

  :global(.ai-markdown tr:hover) {
    background-color: rgba(51, 65, 85, 0.4);
  }

  /* Headings & bold */
  :global(.ai-markdown h1, .ai-markdown h2, .ai-markdown h3, .ai-markdown h4) {
    font-weight: 700;
    color: #f8fafc;
    margin-top: 1.25rem;
    margin-bottom: 0.5rem;
  }

  :global(.ai-markdown strong, .ai-markdown b) {
    color: #38bdf8; /* sky-400 */
    font-weight: 700;
  }

  /* Inline code & pre block styling */
  :global(.ai-markdown :not(pre) > code) {
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.825em;
    padding: 0.15rem 0.4rem;
    border-radius: 0.375rem;
    background-color: #1e293b;
    color: #38bdf8; /* sky-400 */
    border: 1px solid #334155;
  }

  :global(.ai-markdown pre) {
    background-color: #020617;
    color: #e2e8f0;
    border: 1px solid #1e293b;
    border-radius: 0.75rem;
    padding: 0.875rem 1rem;
    overflow-x: auto;
    font-size: 0.8125rem;
    line-height: 1.6;
    margin-top: 0.75rem;
    margin-bottom: 0.75rem;
  }

  /* Lists */
  :global(.ai-markdown ul) {
    list-style-type: disc;
    margin-top: 0.5rem;
    margin-bottom: 0.5rem;
    padding-left: 1.5rem;
  }

  :global(.ai-markdown ol) {
    list-style-type: decimal;
    margin-top: 0.5rem;
    margin-bottom: 0.5rem;
    padding-left: 1.5rem;
  }

  :global(.ai-markdown li) {
    margin-top: 0.25rem;
    margin-bottom: 0.25rem;
    line-height: 1.6;
  }

  /* Paragraphs and line breaks */
  :global(.ai-markdown p) {
    margin-top: 0.5rem;
    margin-bottom: 0.5rem;
    line-height: 1.6;
  }

  /* Horizontal rule */
  :global(.ai-markdown hr) {
    margin-top: 1.25rem;
    margin-bottom: 1.25rem;
    border-top: 1px solid #334155;
  }

  /* Blockquote */
  :global(.ai-markdown blockquote) {
    border-left: 4px solid #06b6d4; /* cyan-500 */
    padding-left: 1rem;
    margin-top: 0.75rem;
    margin-bottom: 0.75rem;
    border-left-color: #22d3ee;
    color: #94a3b8;
    font-style: italic;
  }
</style>
