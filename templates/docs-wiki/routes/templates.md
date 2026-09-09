---
title: "Templates and collections"
navTitle: "Templates"
description: "Wrap content in semantic HTML and generate small navigation or listing structures from nearby pages."
section: "Structure"
order: 3
---

## The content slot

A template is ordinary HTML with exactly one `{{slot}}`. That slot is replaced by the rendered Markdown page.

```html
<!doctype html>
<html lang="en">
    <head>
        <title>{{meta.title}}</title>
    </head>
    <body>
        <main>{{slot}}</main>
    </body>
</html>
```

A template applies to its directory and every directory below it. Add another `template.html` deeper in the tree when a section needs a different frame.

## List nearby pages

An `each` block reads the Markdown pages in a directory. Here, the pages are ordered by a numeric frontmatter value:

```html
<nav>
    {{#each . as page sort order asc}}
        <a href="{{page._url}}">{{page.title}}</a>
    {{/each}}
</nav>
```

Use `./articles` for a child directory or `../reference` for a neighbouring one. Collection values are escaped before they enter the HTML.

## Keep templates simple

The language intentionally has no conditions, filters, includes, or arbitrary scripting. Use HTML for structure, CSS for presentation, and frontmatter for the few values a page needs to expose.
