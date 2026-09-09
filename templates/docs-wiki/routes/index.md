---
title: "Getting started"
navTitle: "Getting started"
description: "Build a small static site from Markdown and ordinary HTML in a few minutes."
section: "Introduction"
order: 1
---

## Install

SSG is a single command-line program. Put the executable on your path, create an empty directory, and scaffold a project:

```sh
mkdir my-site
cd my-site
ssg create
```

Choose a template and a deployment target when prompted. Selecting **None / VPS** creates only the site files.

## Make the first build

Run the development server from the project directory:

```sh
ssg dev
```

Open the printed local address. Changes inside `routes/` rebuild automatically, and CSS edits appear without a full page reload.

## Project shape

A new site stays deliberately small:

```text
routes/
├── index.md
├── guide.md
├── styles.css
└── template.html
```

Markdown becomes HTML, static files are copied, and `template.html` supplies the page frame.
