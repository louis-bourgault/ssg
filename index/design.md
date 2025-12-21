
 INDEXING SYSTEM
 
 for {each} blocks to work, we need a way to index files. We detect properties that every file in that directory has in common in the yaml frontmatter, and then create a json index file that contains an array of all the files

 Schema:

  {
  directory: "/path/to/directory",
  properties: {
  "title": "string",
  "date": "date",
  "slug": "string",
 }
  files: [
	{
		"filename": "my-first-post.md", //this is a safe property; you can't declare a yaml property with this name
	  "title": "My First Post",
	  "date": "2023-10-01",
	  "slug": "my-first-post",
		"url": "/my-first-post" //another safe property; generated from the filename or slug
		path: "/path/to/directory/my-first-post.md" //full path to the file

	}
	]
	
 }
 
 this would also work with a type safe system in the end. For example, if all the files have a slug property but one doesn't, the LSP could warn the user that one file is missing a required property.
 IDK how we infer file types, so for now we'll just use strings for everything. In the future, dates are definitely important for ordering posts.
 
 Then we also have a search system. In the template, we could do things like {each ./ as post} but for more complex structures could do things like {each ../*/ as post} which would find all directories with the wildcard system being valid
 
 I would also like to have a way of doing filtering, like {each ./ where post.date > "2023-01-01" as post}, as well as sorting, but there is also the risk then of making it complex and also straying too far from being a simple static site generator. In future, could even add minor query support through LUA or even *shudders* JS, but for now let's keep it simple.
