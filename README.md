# SSG

A simple SSG for markdown files.

## Why?
There are plenty of SSGs out there -- the one I tried first was Hugo. One thing that I didn't like about it was that the templates and the themes were too far abstracted, and I ended up having to look up docs for all the themes I attempted to use, and where they wanted me to put my files. I want to just make a simple website, not deal with this.
At this point, I could have found another SSG that aligned with what I wanted. Or, you know, I could not bother and make my own from scratch (well, not really -- it is just a 200 line wrapper around Goldmark, which does all the actual work.)

## Syntax:

- ```{{slot}}```: the place where we will insert the generated markdown content
- ```{{meta.*}}```: use this to insert page metadata, which is placed at the top of the markdown file in standard yaml format. Note that only simple properties are compatible for now, with no arrays or objects. If one is passed, you are at the mercy of ```fmt.Sprintf``` as to how it makes formats it.


I aim to add further syntax. One I am looking at implementing is an each system, that can be used to show different things within a directory. It would follow a syntax like:
```{#each . as item sort date desc}```.

## Files and how it works

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
- image optimising and lazy loading
- dev server
- preload on hover (optional postprocessing system)
- {each} tags as above
- change some parsing from weird ```strings.Split``` hacky solutions to using regex properly
