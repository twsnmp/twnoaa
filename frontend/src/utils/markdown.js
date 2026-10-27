import { marked } from "marked";
import DOMPurify from "dompurify";

// Configure marked defaults for AI outputs (GFM tables, breaks for newlines)
marked.setOptions({
  gfm: true,
  breaks: true,
});

// Configure DOMPurify hook to ensure links safely open in a new tab
if (typeof window !== "undefined" && DOMPurify.addHook) {
  DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (node.tagName === "A" && node.hasAttribute("href")) {
      node.setAttribute("target", "_blank");
      node.setAttribute("rel", "noopener noreferrer");
    }
  });
}

/**
 * Parses markdown into sanitized HTML safe for direct rendering.
 *
 * @param {string | null | undefined} content - Raw markdown text
 * @returns {string} Sanitized HTML string safe for {@html ...}
 */
export function renderMarkdown(content) {
  if (!content || !content.trim()) {
    return "";
  }

  try {
    // Parse markdown synchronously with GFM tables and line-breaks
    const rawHtml = marked.parse(content, { async: false, breaks: true, gfm: true });

    // Sanitize with DOMPurify in browser environment
    if (typeof window !== "undefined" && DOMPurify.sanitize) {
      return DOMPurify.sanitize(rawHtml, {
        USE_PROFILES: { html: true },
        ADD_ATTR: ["target", "rel"],
      });
    }

    return rawHtml;
  } catch (err) {
    console.error("Failed to parse AI markdown:", err);
    return content;
  }
}
