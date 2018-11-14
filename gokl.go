package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	git "gopkg.in/src-d/go-git.v4"
)

const datelayout = "2006-01-02"
const outputdate = "Monday, 2. January 2006"

type logentry struct {
	begin    time.Time
	end      time.Time
	topic    string
	appendix string
	media    []string
	body     string
}

type outputentry struct {
	Begin    string
	End      string
	Topic    string
	Appendix string
	Media    []string
	Body     string
}

type outputpage struct {
	Entries    []outputentry
	MediaLinks []string
	Links      []string
	Month      string
	Year       string
}

type ByBegin []logentry

func (a ByBegin) Len() int           { return len(a) }
func (a ByBegin) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByBegin) Less(i, j int) bool { return a[i].begin.Before(a[j].begin) }

func getRepo(repodir string, repourl string) error {
	var repo *git.Repository
	var err error
	repo, err = git.PlainClone(repodir, false, &git.CloneOptions{
		URL:      repourl,
		Progress: os.Stdout,
	})
	if err != nil {
		if err != git.ErrRepositoryAlreadyExists {
			return errors.New("Error while cloning repo:" + err.Error())
		}
		repo, err = git.PlainOpen(repodir)
		if err != nil {
			return errors.New("Error while opening repo:" + err.Error())
		}
	}
	tree, err := repo.Worktree()
	if err != nil {
		return errors.New("Error while creating worktree:" + err.Error())
	}
	err = tree.Reset(&git.ResetOptions{
		Mode: git.HardReset,
	})
	if err != nil {
		return errors.New("Error while resetting repo:" + err.Error())
	}
	err = tree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Force:      true,
	})
	if err != nil {
		if err != git.NoErrAlreadyUpToDate {
			return errors.New("Error while pulling changes:" + err.Error())
		}
	}
	return nil
}

func parseFile(filepath string) (logentry, error) {
	var result logentry

	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		return result, err
	}
	content := string(b)
	split := strings.Split(content, "\n\n")
	if len(split) != 2 {
		return result, errors.New("Invalid entry format")
	}
	header := split[0]
	body := split[1]

	split = strings.Split(header, "\n")
	for _, s := range split {
		if !strings.HasPrefix(s, "#") {
			split := strings.SplitN(s, ": ", 2)
			switch split[0] {
			case "BEGIN":
				t, err := time.Parse(datelayout, split[1])
				if err != nil {
					return result, errors.New("Error while parsing date:" + err.Error())
				}
				result.begin = t

			case "END":
				if split[1] != "None" {
					t, err := time.Parse(datelayout, split[1])
					if err != nil {
						return result, errors.New("Error while parsing date:" + err.Error())
					}
					result.end = t
				}
			case "TOPIC":
				result.topic = split[1]
			case "APPENDIX":
				result.appendix = split[1]
			case "MEDIA":
				split := strings.Split(split[1], ",")
				result.media = append(result.media, split[0])
			}
		}
	}
	result.body = body

	return result, nil
}

func generateLogEntries(repodir string) ([]logentry, error) {
	var result []logentry

	fis, err := ioutil.ReadDir(repodir)
	if err != nil {
		return result, err
	}

	for _, fi := range fis {
		if fi.Name() != "media" {
			if fi.IsDir() {
				yeardir := repodir + string(filepath.Separator) + fi.Name()
				fis2, err := ioutil.ReadDir(yeardir)
				if err != nil {
					return result, err
				}
				for _, fi2 := range fis2 {
					if fi2.IsDir() {
						monthdir := yeardir + string(filepath.Separator) + fi2.Name()
						fis3, err := ioutil.ReadDir(monthdir)
						if err != nil {
							return result, err
						}
						for _, fi3 := range fis3 {
							if !fi3.IsDir() {
								path := monthdir + string(filepath.Separator) + fi3.Name()
								le, err := parseFile(path)
								if err != nil {
									return nil, err
								}
								result = append(result, le)
							}
						}
					}
				}
			}
		}
	}

	return result, nil
}

func convertlink(in string, count int) (string, string) {
	var inlink, listlink string
	status := 0
	scount := strconv.Itoa(count)
	enclosed := strings.Replace(in, "]]", "", -1)

	if strings.Contains(in, "[[:") {
		status = 2
		enclosed = strings.Replace(enclosed, "[[:", "", -1)
	} else {
		enclosed = strings.Replace(enclosed, "[[", "", -1)
		if strings.Contains(enclosed, "http://") || strings.Contains(enclosed, "https://") {
			status = 3
		} else {
			status = 2
		}
	}

	split := strings.Split(enclosed, "|")
	link := split[0]
	name := ""
	if len(split) > 1 {
		name = split[1]
	} else {
		name = link
	}

	switch status {
	case 2:
		inlink = "[" + name + "][LINK:" + scount + "]"
		listlink = "[h|" + "[LINK " + scount + "]: " + name + "|" + "URL:http://www.binary-kitchen.de/wiki/doku.php?id=" + link + "|gopher.binary-kitchen.de|70]"
	case 3:
		inlink = "[" + name + "][LINK:" + scount + "]"
		listlink = "[h|" + "[LINK " + scount + "]: " + name + "|" + "URL:" + link + "|gopher.binary-kitchen.de|70]"

	}

	return inlink, listlink
}

func formatEntry(entry logentry, imageurl string, linkcount, mediacount int) (outputentry, []string, []string, int, int) {
	var outentry outputentry
	var links []string
	var media []string

	outentry.Begin = entry.begin.Format(outputdate)
	if entry.end.After(entry.begin) {
		outentry.End = "bis " + entry.end.Format(outputdate)
	}
	outentry.Topic = entry.topic
	outentry.Appendix = entry.appendix
	outentry.Body = entry.body
	for strings.Contains(outentry.Body, "[[") {
		sep := strings.SplitN(outentry.Body, "[[", 2)
		if len(sep) > 1 {
			before := sep[0]
			remaining := sep[1]
			sep := strings.SplitAfterN(remaining, "]]", 2)
			if len(sep) > 1 {
				middle := sep[0]
				after := sep[1]
				inlink, listlink := convertlink(middle, linkcount)
				links = append(links, listlink)
				outentry.Body = before + inlink + after
				linkcount = linkcount + 1
			}
		}
	}
	for _, m := range entry.media {
		imagename := "[BILD " + strconv.Itoa(mediacount) + "]"
		outentry.Media = append(outentry.Media, imagename)

		imagelink := "[h|" + imagename + "|URL:" + imageurl + m + "|gopher.binary-kitchen.de|70]"
		media = append(media, imagelink)
		mediacount = mediacount + 1
	}

	return outentry, links, media, linkcount, mediacount
}

func generateGopherDir(entries []logentry, gopherdir string, imageurl string, templatepath string) error {
	month := ""
	mediacount := 1
	linkcount := 1
	var currentpage outputpage
	for index, e := range entries {
		newmonth := e.begin.Format("01-January")
		if month != newmonth {
			year := e.begin.Format("2006")
			currentpage.Month = month
			if newmonth == "01-January" {
				year = entries[index-1].begin.Format("2006")
			}
			currentpage.Year = year
			monthpath := gopherdir + string(filepath.Separator) + year + string(filepath.Separator) + month
			err := os.MkdirAll(monthpath, 0755)
			if err != nil {
				return errors.New("Error creating month directory: " + err.Error())
			}
			t, err := template.ParseFiles(templatepath)
			if err != nil {
				return errors.New("Error parsing tempalte: " + err.Error())
			}
			f, err := os.Create(monthpath + string(filepath.Separator) + "index.gph")
			if err != nil {
				return errors.New("Error creating gopherfile: " + err.Error())
			}
			defer f.Close()
			err = t.Execute(f, &currentpage)
			if err != nil {
				return errors.New("Error executing template: " + err.Error())
			}
			f.Close()
			currentpage = outputpage{}
			mediacount = 1
			linkcount = 1
			month = newmonth
		}
		outentry, links, media, newlinkcount, newmediacount := formatEntry(e, imageurl, linkcount, mediacount)
		currentpage.Entries = append(currentpage.Entries, outentry)
		currentpage.Links = append(currentpage.Links, links...)
		currentpage.MediaLinks = append(currentpage.MediaLinks, media...)
		linkcount = newlinkcount
		mediacount = newmediacount
	}

	return nil
}

func main() {
	var repodir string
	var gopherdir string
	var repourl string
	var imageurl string
	var templatepath string
	flag.StringVar(&repodir, "r", "./", "Directory for checking out the repository")
	flag.StringVar(&gopherdir, "g", "/var/gopher/Kuechenlog", "Directory for the generated gopher content")
	flag.StringVar(&repourl, "u", "https://github.com/Binary-Kitchen/kitchenlog.git", "URL of the log repository")
	flag.StringVar(&imageurl, "i", "https://raw.githubusercontent.com/Binary-Kitchen/kitchenlog/master/media/", "The URL for the raw image files")
	flag.StringVar(&templatepath, "t", "./month-template.txt", "Path to the template for the gopher pages")
	flag.Parse()

	log.Println("Getting Repository")
	err := getRepo(repodir, repourl)
	if err != nil {
		log.Fatal("Error getting Repo:", err)
	}
	log.Println("Parsing Log Entries")
	les, err := generateLogEntries(repodir)
	if err != nil {
		log.Fatal("Error parsing Entries:", err)
	}
	sort.Sort(ByBegin(les))
	log.Println("Wirting in gopher dir")
	err = generateGopherDir(les, gopherdir, imageurl, templatepath)
	if err != nil {
		log.Fatal("Error creating gopher files:", err)
	}
	log.Println("Done")
}
