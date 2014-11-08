package main

import (
	"bufio"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/howeyc/fsnotify"
	"github.com/russross/blackfriday"
	"github.com/yosssi/ace"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const frontMatterDelim = "+++\n"

const (
	sourceDir     = "source"
	destDir       = "public"
	postsDir      = "source/_posts"
	sassSourceDir = "source/_sass"
	sassDestDir   = "public/css"
)

type Post struct {
	Title       string        `toml:"title"`
	Author      string        `toml:"author"`
	Description string        `toml:"description"`
	Content     template.HTML `toml:"-"`
	Url         string        `toml:"-"`
	Dir         string        `toml:"-"`
}

type Site struct {
	Title       string
	Author      string
	Description string
}

type Context struct {
	Site  Site
	Posts []Post
	Post  Post
}

var context = Context{
	Site: Site{
		Title:       "Alex Browne's Blog",
		Author:      "Alex Browne",
		Description: "A blog written by Alex Browne about coding and shit.",
	},
	Posts: []Post{},
}

func main() {
	generate()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	done := make(chan bool)

	// Process events
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				base := filepath.Base(ev.Name)
				if base[0] != '.' {
					// ignore hidden files
					fmt.Println("generating...")
					generate()
				}
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Watch(sourceDir + "/_posts")
	if err != nil {
		log.Fatal(err)
	}

	runSass()

	<-done
	watcher.Close()
}

func generate() {
	parsePosts()
	generateIndex()
	generatePosts()
}

func runSass() {
	cmd := exec.Command("sass", "--watch", fmt.Sprintf("%s:%s", sassSourceDir, sassDestDir))
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func parsePosts() {
	// remove any old posts
	context.Posts = []Post{}
	// walk through the source/posts dir
	if err := filepath.Walk(postsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// check if markdown file (ignore everything else)
		if filepath.Ext(path) == ".md" {
			// create a new Post object from the file and append it to context.Posts
			p, err := createPostFromPath(path, info)
			if err != nil {
				return err
			}
			context.Posts = append(context.Posts, p)
		}
		return nil
	}); err != nil {
		panic(err)
	}
}

func generateIndex() {
	// create the file
	file, err := os.Create(destDir + "/index.html")
	if err != nil {
		log.Fatal(err)
	}

	// load and execute the template for the index page
	tpl, err := ace.Load("base", "index", &ace.Options{BaseDir: sourceDir + "/_templates"})
	if err != nil {
		log.Fatal(err)
	}
	if err := tpl.Execute(file, context); err != nil {
		log.Fatal(err)
	}
}

func generatePosts() {
	// load the template
	tpl, err := ace.Load("base", "post", &ace.Options{BaseDir: sourceDir + "/_templates"})
	if err != nil {
		panic(err)
	}
	for _, p := range context.Posts {
		p.generate(tpl)
	}
}

func (p Post) generate(tpl *template.Template) {
	dirName := destDir + "/" + p.Dir

	// make the directory for each post
	err := os.Mkdir(dirName, os.ModePerm|os.ModeDir)
	if err != nil {
		// if the directory already exists, that's fine
		// if there was some other error, panic
		if !os.IsExist(err) {
			panic(err)
		}
	}

	// make an index.html file inside that directory
	file, err := os.Create(dirName + "/index.html")
	if err != nil {
		// if the file already exists, that's fine
		// if there was some other error, panic
		if !os.IsExist(err) {
			panic(err)
		}
	}
	context.Post = p
	if err := tpl.Execute(file, context); err != nil {
		panic(err)
	}
}

func createPostFromPath(path string, info os.FileInfo) (Post, error) {
	// create post object
	name := strings.TrimSuffix(info.Name(), filepath.Ext(path))
	p := Post{
		Url: "/" + name,
		Dir: name,
	}

	// open the source file
	file, err := os.Open(path)
	if err != nil {
		return p, err
	}

	// extract and parse front matter
	if err := p.parseFromFile(file); err != nil {
		return p, err
	}

	return p, nil
}

func (p *Post) parseFromFile(file *os.File) error {
	r := bufio.NewReader(file)
	frontMatter, content, err := split(r)
	if err != nil {
		return err
	}
	if _, err := toml.Decode(frontMatter, p); err != nil {
		return err
	}
	p.Content = template.HTML(blackfriday.MarkdownCommon([]byte(content)))
	return nil
}

func split(r *bufio.Reader) (frontMatter string, content string, err error) {
	if containsFrontMatter(r) {
		// split the file into two pieces according to where we
		// find the closing delimiter
		frontMatter := ""
		content := ""
		scanner := bufio.NewScanner(r)
		// skip first line because it's just the delimiter
		scanner.Scan()
		// whether or not we have reached content portion yet
		reachedContent := false
		for scanner.Scan() {
			if line := scanner.Text() + "\n"; line == frontMatterDelim {
				// we have reached the closing delimiter, everything
				// else in the file is content.
				reachedContent = true
			} else {
				if reachedContent {
					content += line
				} else {
					frontMatter += line
				}
			}
		}
		return frontMatter, content, nil
	} else {
		// there is no front matter
		if content, err := ioutil.ReadAll(r); err != nil {
			return "", "", err
		} else {
			return "", string(content), nil
		}
	}
}

func containsFrontMatter(r *bufio.Reader) bool {
	firstBytes, err := r.Peek(4)
	if err != nil {
		panic(err)
	}
	return string(firstBytes) == frontMatterDelim
}
