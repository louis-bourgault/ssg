---
title: "Build and publish"
navTitle: "Build and publish"
description: "Create a production build and send the resulting static directory to any ordinary web host."
section: "Deployment"
order: 4
---

## Build for production

Run SSG without a subcommand:

```sh
ssg
```

The complete site is written to `build/`. A failed build leaves the previous successful output intact.

## Choose a host

The output is plain static HTML, CSS, and assets. It can be served by a web server, an object store, or a static hosting service.

If you selected a provider while running `ssg create`, the project already contains the small build script and configuration file that provider needs.

## Pin a release

Provider scripts use the latest SSG release by default. Set `SSG_VERSION` to an explicit release tag when repeatable deploys matter:

```sh
SSG_VERSION=v0.1.0 ./vercel_build.sh
```

Commit the provider files beside the routes so deployment configuration changes with the site.
