# SSG

SSG builds a site from ordinary HTML templates and Markdown files. Its template language intentionally has only one content slot, metadata values, directory loops, optional sorting, and generated headings.

Put source files in `routes/` and run:

```sh
go run .
```

Markdown pages get pretty output paths: `routes/about.md` becomes `build/about/index.html`, `routes/posts/index.md` becomes `build/posts/index.html`, and `routes/index.md` becomes `build/index.html`. Other files are copied to the corresponding location under `build/`.

## Templates

A `template.html` is ordinary HTML with exactly one Markdown slot:

```html
<!doctype html>
<html>
    <body>{{slot}}</body>
</html>
```

The template applies to Markdown files in its directory and all descendant directories. A nearer `template.html` overrides an inherited one. Template files are never copied to the output. If there is no template, SSG uses a minimal default template.

Template directives are interpreted only in `template.html`, never inside Markdown. This makes text such as `{{meta.example}}` safe to use literally in a Markdown document.

## Metadata

Frontmatter properties are available through `meta`:

```md
---
Title: About us
description: What we make
---
```

```html
<title>{{meta.Title}}</title>
<meta name="description" content="{{meta.description}}">
{{slot}}
```

Strings, numbers, and booleans can be rendered. Arrays and objects may exist in frontmatter but cannot be interpolated directly. A missing property is a build error, which catches spelling mistakes instead of silently producing an empty value.

## Directory collections

Use one `each` block to list the Markdown pages in a directory:

```html
{{#each . as item}}
    <a href="{{item._url}}">{{item.Title}}</a>
{{/each}}
```

`.` means the current Markdown page's directory. Relative paths select another directory: `./posts` is a child and `../posts` is parent-relative. Paths cannot escape `routes/`, and globs are not supported.

The name after `as` is the alias for that block, so descriptive aliases are encouraged:

```html
{{#each ./posts as post}}
    <a href="{{post._url}}">{{post.title}}</a>
{{/each}}
```

Without an explicit sort, pages are ordered by slash-normalized source path. Sort on a frontmatter or built-in property with `sort`; direction defaults to `asc`:

```html
{{#each ./posts as post sort date desc}}
    <article>
        <a href="{{post._url}}">{{post.title}}</a>
        <time>{{post.date}}</time>
    </article>
{{/each}}
```

String values sort lexicographically and numbers sort numerically. Missing sort values and incompatible mixed types are build errors. Equal values use source path as a deterministic tie-breaker.

Each page exposes its frontmatter directly through the alias plus these built-ins:

- `{{item._url}}` — final pretty URL, such as `/posts/hello/`
- `{{item._filename}}` — source filename, such as `hello.md`
- `{{item._path}}` — slash-separated path relative to `routes/`, such as `posts/hello.md`
- `{{item._preview100}}` — cached plain-text preview, truncated to 100 Unicode characters

Preview lengths are non-negative integers. `...` is appended only if text was actually truncated. Frontmatter names beginning with `_` are reserved by SSG.

Blocks can be nested, and aliases follow their block scope. The language has no conditions, expressions, filters, pagination, includes, functions, or scripting.

## Generated headings

SSG collects level 1–6 headings while Goldmark renders the current Markdown document. Templates decide whether and how to display them:

```html
<nav aria-label="On this page">
    {{#each meta.headings as heading}}
        <a href="#{{heading.id}}">
            {{heading.text}}
        </a>
    {{/each}}
</nav>
```

Heading values are `heading.level`, `heading.text`, and `heading.id`. IDs exactly match Goldmark's emitted IDs, text contains no HTML tags, and headings remain in document order. `meta.headings` is computed and cannot be declared in frontmatter.

## Escaping and errors

Metadata, collection properties, headings, URLs, filenames, paths, and previews are HTML-escaped. `{{slot}}` is the only trusted HTML value because it contains HTML produced by Goldmark. There is no raw interpolation form.

Template syntax and rendering errors include the template path, line, and column. Every custom template must have exactly one top-level `{{slot}}`; slots inside loops, unknown directives, malformed loop headers, and unclosed blocks are rejected.

## Links and images

After rendering, relative `href` and `src` values are rewritten to their production paths. Supported raster images can receive generated responsive variants and `srcset` attributes. SVG files are left unchanged.

Image conversion uses libwebp through CGO. On Debian or Ubuntu, install it with:

```sh
sudo apt-get install libwebp-dev
```

## Initialising

Use [degit](https://github.com/Rich-Harris/degit), by Rich Harris, or copy the template directory of this repository:

```sh
degit louis-bourgault/ssg/template
```

## Development server

Run the development server from the project root:

```sh
go run . dev [--host HOST] [--port PORT] [--output DIRECTORY]
```

The defaults are `--host 127.0.0.1`, `--port 8080`, and `--output .ssg-dev`. Use port `0` to select an available ephemeral port. Binding to a LAN or public interface, such as `--host 0.0.0.0`, must be requested explicitly.

Development uses the same complete build pipeline as production, but writes into `.ssg-dev` instead of `build`. One recursive watcher monitors the entire `routes` tree, combines rapid changes into a single rebuild, and detects changed, created, renamed, and deleted pages, templates, collection entries, styles, and assets. Successful non-CSS builds reload every connected browser; CSS-only builds refresh matching stylesheets without reloading the page. Build failures leave the last successful output intact and appear in a dismissible browser overlay. Fixing the source clears the error and reloads automatically. The server also reconnects dropped browser connections and shuts down cleanly on Ctrl+C.
