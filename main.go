package main

import (
	"bufio"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/albrow/ace"
	"github.com/howeyc/fsnotify"
	"github.com/russross/blackfriday"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const frontMatterDelim = "+++\n"

var (
	sourceDir, destDir, postsDir, sassSourceDir, sassDestDir string
)

type Post struct {
	Title       string        `toml:"title"`
	Author      string        `toml:"author"`
	Description string        `toml:"description"`
	Content     template.HTML `toml:"-"`
	Url         string        `toml:"-"`
	Dir         string        `toml:"-"`
}

type Context map[string]interface{}

var (
	context = Context{}
	posts   = []Post{}
)

func main() {
	parseConfig()
	generate()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	watch(watcher)
}

func parseConfig() {
	if _, err := toml.DecodeFile("config.toml", context); err != nil {
		panic(err)
	}
	vars := map[string]*string{
		"sourceDir":     &sourceDir,
		"destDir":       &destDir,
		"postsDir":      &postsDir,
		"sassSourceDir": &sassSourceDir,
		"sassDestDir":   &sassDestDir,
	}
	setGlobalConfig(vars, context)
}

func setGlobalConfig(vars map[string]*string, data map[string]interface{}) {
	for name, holder := range vars {
		if value, found := data[name]; found {
			(*holder) = fmt.Sprint(value)
		}
	}
}

func generate() {
	removeOld()
	parsePosts()
	generatePages()
	generatePosts()
}

func removeOld() {
	// walk through the dest dir
	if err := filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == destDir {
			// ignore the destDir itself
			return nil
		} else if info.IsDir() {
			if path == sassDestDir {
				// let sass handle this one
				return filepath.SkipDir
			}
			// remove the dir and everything in it
			if err := os.RemoveAll(path); err != nil {
				panic(err)
			}
			return filepath.SkipDir
		} else {
			// remove the file
			if err := os.Remove(path); err != nil {
				panic(err)
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
}

func watch(watcher *fsnotify.Watcher) {
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

	err := watcher.Watch(postsDir)
	if err != nil {
		log.Fatal(err)
	}
	runSass()

	<-done
}

func runSass() {
	cmd := exec.Command("sass", "--watch", fmt.Sprintf("%s:%s", sassSourceDir, sassDestDir))
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

func generatePages() {
	// walk through the source dir
	if err := filepath.Walk(sourceDir, func(innerPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		base := info.Name()
		if base[0] == '.' || base[0] == '_' {
			// ignore two kinds of files
			// 1. those that start with a '.' are hidden system files
			// 2. those that start with a '_' are specifically ignored by scribble
			if info.IsDir() {
				// skip any files in directories that start with '_'
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			ext := filepath.Ext(base)
			switch ext {
			case ".ace":
				generatePageFromPath(innerPath)
			default:
				// copy the file directly to the destDir
				destPath := strings.Replace(innerPath, sourceDir, destDir, 1)
				srcFile, err := os.Open(innerPath)
				if err != nil {
					panic(err)
				}
				if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
					panic(err)
				}
				destFile, err := os.Create(destPath)
				if err != nil {
					panic(err)
				}
				if _, err := io.Copy(destFile, srcFile); err != nil {
					panic(err)
				}
			}
		}
		return nil
	}); err != nil {
		panic(err)
	}
}

func generatePageFromPath(path string) {
	srcFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	reader := bufio.NewReader(srcFile)
	frontMatter, content, err := split(reader)
	pageContext := context
	if frontMatter != "" {
		if _, err := toml.Decode(frontMatter, pageContext); err != nil {
			panic(err)
		}
	}
	layout := "base"
	if otherLayout, found := pageContext["layout"]; found {
		layout = otherLayout.(string)
	}
	tpl, err := ace.Load("_layouts/"+layout, filepath.Base(path), &ace.Options{
		BaseDir: sourceDir,
		Asset: func(name string) ([]byte, error) {
			return []byte(content), nil
		},
	})
	if err != nil {
		panic(err)
	}
	destPath := strings.Replace(path, sourceDir, destDir, 1)
	destPath = strings.Replace(destPath, ".ace", ".html", 1)
	if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
		panic(err)
	}
	destFile, err := os.Create(destPath)
	if err != nil {
		// if the file already exists, that's fine
		// if there was some other error, panic
		if !os.IsExist(err) {
			panic(err)
		}
	}
	if err := tpl.Execute(destFile, pageContext); err != nil {
		panic(err)
	}
}

func parsePosts() {
	// remove any old posts
	posts = []Post{}
	context["Posts"] = posts
	// walk through the source/posts dir
	if err := filepath.Walk(postsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// check if markdown file (ignore everything else)
		if filepath.Ext(path) == ".md" {
			// create a new Post object from the file and append it to posts
			p, err := createPostFromPath(path, info)
			if err != nil {
				return err
			}
			posts = append(posts, p)
		}
		context["Posts"] = posts
		return nil
	}); err != nil {
		panic(err)
	}
}

func generatePosts() {
	// load the template
	tpl, err := ace.Load("_layouts/base", "_views/post", &ace.Options{BaseDir: sourceDir})
	if err != nil {
		panic(err)
	}
	for _, p := range posts {
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
	context["Post"] = p
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
