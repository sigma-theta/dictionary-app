package main

import (
	"html/template"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Templates struct {
	templates *template.Template
}

func (t *Templates) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func newTemplate() *Templates {
	return &Templates{
		templates: template.Must(template.ParseGlob("../views/*.html")),
	}
}

type Count struct {
	Count int
}

var id = 0

type Word struct {
	Original    string
	Translation string
	Id          int
}

func newWord(original, translation string) Word {
	id++
	return Word{
		Id:          id,
		Original:    original,
		Translation: translation,
	}
}

type Words = []Word

type Data struct {
	Words         Words
	SearchResults Words
}

func (d *Data) indexOf(id int) int {
	for i, word := range d.Words {
		if word.Id == id {
			return i
		}
	}
	return -1
}

func (d *Data) wordExists(original string) bool {
	for _, word := range d.Words {
		if word.Original == original {
			return true
		}
	}
	return false
}

func newData() Data {
	return Data{
		Words: []Word{
			newWord("aanpassen", "to adapt"),
			newWord("aantonen", "to demonstrate"),
		},
		SearchResults: []Word{},
	}
}

type FormData struct {
	Values map[string]string
	Errors map[string]string
}

func newFormData() FormData {
	return FormData{
		Values: make(map[string]string),
		Errors: make(map[string]string),
	}
}

type Page struct {
	Data Data
	Form FormData
}

func newPage() Page {
	return Page{
		Data: newData(),
		Form: newFormData(),
	}
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())

	page := newPage()

	e.Renderer = newTemplate()
	e.Static("/css", "css") //not used right now, will fix maybe

	e.GET("/", func(c echo.Context) error {
		return c.Render(200, "index", page)
	})

	e.POST("/words", func(c echo.Context) error {
		original := c.FormValue("original")
		translation := c.FormValue("translation")

		if page.Data.wordExists(original) {
			formData := newFormData()
			formData.Values["original"] = original
			formData.Values["translation"] = translation
			formData.Errors["original"] = "Word already exists"

			return c.Render(422, "word-entry", formData)
		}

		defn := newWord(original, translation)
		page.Data.Words = append(page.Data.Words, defn)

		c.Render(200, "word-entry", newFormData())

		return c.Render(200, "oob-word", defn)
	})

	e.POST("/search", func(c echo.Context) error {
		keyword := c.FormValue("search")

		if keyword == "" {
			page.Data.SearchResults = nil
		} else {
			keyword = strings.ToLower(keyword)

			page.Data.SearchResults = make([]Word, 0, len(page.Data.Words))
			for _, word := range page.Data.Words {
				if strings.Contains(strings.ToLower(word.Original), keyword) || strings.Contains(strings.ToLower(word.Translation), keyword) {
					page.Data.SearchResults = append(page.Data.SearchResults, word)
				}
			}
		}

		return c.Render(200, "search-results", page.Data)
	})

	e.DELETE("/words/:id", func(c echo.Context) error {
		time.Sleep(1 * time.Second)
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)

		if err != nil {
			return c.String(400, "Invalid id")
		}

		index := page.Data.indexOf(id)
		if index == -1 {
			return c.String(404, "Contact not found")
		}

		page.Data.Words = append(page.Data.Words[:index], page.Data.Words[index+1:]...)

		return c.NoContent(200)
	})

	e.Logger.Fatal(e.Start(":42069"))
}
