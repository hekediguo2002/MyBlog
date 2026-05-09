import '../../vendor/marked.min.js';
import '../../vendor/highlight.min.js';

const { marked } = window;

marked.setOptions({
  breaks: true,
  headerIds: true,
});

export function renderMarkdown(content) {
  const html = marked.parse(content || '');
  const div = document.createElement('div');
  div.innerHTML = html;

  div.querySelectorAll('script').forEach(el => el.remove());
  div.querySelectorAll('*').forEach(el => {
    for (const attr of Array.from(el.attributes)) {
      if (attr.name.startsWith('on')) el.removeAttribute(attr.name);
    }
  });

  div.querySelectorAll('pre code').forEach(block => {
    if (window.hljs) window.hljs.highlightElement(block);
  });

  return div.innerHTML;
}
