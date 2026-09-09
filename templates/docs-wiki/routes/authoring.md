---
title: "Pages and content"
navTitle: "Pages and content"
description: "Write Markdown pages, attach metadata, and link between routes without managing output paths."
section: "Authoring"
order: 2
---

## Add a page

Every Markdown file below `routes/` becomes a page. A source file at `routes/about.md` is written to `/about/index.html`, giving it the public URL `/about/`.

Start a page with YAML frontmatter:

```md
---
title: "About the project"
description: "Why this project exists."
---

The page begins here.
```

Metadata is available to the nearest `template.html` through expressions such as `{{meta.title}}`.

## Link pages and assets

Write links relative to the Markdown source. SSG turns the source path into the final public URL:

```md
[Read the guide](./guide.md)
![A diagram](./images/diagram.png)
```

Files that are not Markdown or `template.html` are copied to the equivalent output location.

## Use headings

Normal Markdown headings receive stable IDs. Templates can also read them from `meta.headings` to make a table of contents like the one on this page.
