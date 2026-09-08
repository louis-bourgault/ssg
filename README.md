# SSG

A simple SSG for markdown files.

## Why?
There are plenty of SSGs out there -- the one I tried first was Hugo. One thing that I didn't like about it was that the templates and the themes were too far abstracted, and I ended up having to look up docs for all the themes I attempted to use, and where they wanted me to put my files. I want to just make a simple website, not deal with this.
At this point, I could have found another SSG that aligned with what I wanted. Or, you know, I could not bother and make my own from scratch (well, not really -- it is just a 200 line wrapper around Goldmark, which does all the actual work.)

## Syntax:

- ```{{slot}}```: the place where we will insert the generated markdown content
- ```{{meta.*}}```: use this to insert page metadata, which is placed at the top of the markdown file in standard yaml format. Note that only simple properties are compatible for now, with no arrays or objects. If one is passed, you are at the mercy of ```fmt.Sprintf``` as to how it makes formats it.


I aim to add further syntax. One I am looking at implementing is an each system, that can be used to show different things within a directory. It would follow a syntax like:
```{#each . as item sort date desc}```. Also, to allow table of contents systems, I could also do ```meta.headings``` which would work with each blocks.

## Files and how it works

Chuck everything in a routes directory. This is what will be built

/routes/{name}.md ==> /build/name/index.html (using the template.html to put it in)
/routes/about/index.md ==> /build/about/index.html (using compilation)
/routes/about/index.html is just copied across
/routes/about.html ==> /routes/about/index.html
Any file that is not ending in md or html is just copied. I do intend to do image processing in the future, so it will get different resolutions of the images to put into ```srcset``` tags in HTML -- but not yet.

## Link Handling
Generated HTML files are postprocessed so any normal markdown link -- in the format of ```[Link_Name](./path/to/link)``` is fixed so that it will link properly. This also handles src tags for images, and any href tag, whether it be for a CSS file or a link.

## Templating

This system works around templates -- you can chuck a ```template.html``` file in a directory, and any files in that directory, as well as child directories that do not have their own template.html file, will be compiled using that. It really is pretty simple.

## What I use it for

Mainly simple wikis and static sites. I want a site for a project -- I can just write some basic html once, then markdown and copy in pico css or simple.css, and I have a site.

## Possible Later Things:

Possible things to add are:
- image optimising and lazy loading (will be really easy, just need to modify img tags)
- dev server
- preload on hover (optional postprocessing system)
- {each} tags as above

## Environment
For webp, we use a library that does conversions through C bindings. Thus, you should use a machine with cgo working to compile and run this. If you're on windows, you could go through the hassle of installing MYS32, but do yourself a favour and just use WSL instead (or delete windows entirely, preferably)
You will need libwebp installed: ```sudo apt-get install libwebp-dev```

# Initialising
Use degit, by Rich Harris, or copy the template directory of this repository.
[https://github.com/Rich-Harris/degit](https://github.com/Rich-Harris/degit)
```degit louis-bourgault/ssg/template```

## Development server

Run the development server from the project root:

```sh
go run . dev [--host HOST] [--port PORT] [--output DIRECTORY]
```

The defaults are `--host 127.0.0.1`, `--port 8080`, and `--output .ssg-dev`. Use port `0` to select an available ephemeral port. Binding to a LAN or public interface, such as `--host 0.0.0.0`, must be requested explicitly.

Development uses the same complete build pipeline as production, but writes into `.ssg-dev` instead of `build`. One recursive watcher monitors the entire `routes` tree, combines rapid changes into a single rebuild, and detects changed, created, renamed, and deleted pages, templates, collection entries, styles, and assets. Successful non-CSS builds reload every connected browser; CSS-only builds refresh matching stylesheets without reloading the page. Build failures leave the last successful output intact and appear in a dismissible browser overlay. Fixing the source clears the error and reloads automatically. The server also reconnects dropped browser connections and shuts down cleanly on Ctrl+C.
