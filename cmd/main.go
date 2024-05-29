package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gwthm-in/dotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
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

type Word struct {
	Original    string
	Translation string
	Id          int
}

func newWord(original, translation string, db *sql.DB) Word {
	newWord := new(Word)
	appendQry := fmt.Sprintf("insert into words(original, meaning) values ('%s', '%s') returning *", original, translation)
	err := db.QueryRow(appendQry).Scan(&newWord.Id, &newWord.Original, &newWord.Translation)
	if err != nil {
		fmt.Printf("couldn't add new word: %s", err)
	}
	return Word{
		Id:          newWord.Id,
		Original:    newWord.Original,
		Translation: newWord.Translation,
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
		Words:         []Word{},
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

func getWords(db *sql.DB, page *Page) {
	page.Data.Words = []Word{}
	rows, err := db.Query("select * from words order by original")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var defn Word
	for rows.Next() {
		err := rows.Scan(&defn.Id, &defn.Original, &defn.Translation)
		if err != nil {
			log.Fatal(err)
		}
		//log.Println(defn.Id, defn.Original)
		page.Data.Words = append(page.Data.Words, Word{Id: defn.Id, Original: defn.Original, Translation: defn.Translation})
	}

	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	dotenv.OptLookupMod()
	err := dotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env: %s", err)
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	conn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=require", dbUser, dbPass, dbHost, dbName)
	db, err := sql.Open("postgres", conn)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	var version string
	if err := db.QueryRow("select version()").Scan(&version); err != nil {
		panic(err)
	}
	fmt.Printf("version=%s\n", version)

	e := echo.New()
	e.Use(middleware.Logger())

	page := newPage()
	getWords(db, &page)

	e.Renderer = newTemplate()
	e.Static("../css", "css") //not used right now, will fix maybe

	e.GET("/", func(c echo.Context) error {
		fmt.Printf("%d words in database", len(page.Data.Words))
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

		defn := newWord(original, translation, db)
		page.Data.Words = append([]Word{defn}, page.Data.Words...)

		c.Render(200, "word-entry", newFormData())

		return c.Render(200, "table", page.Data)
	})

	e.POST("/search", func(c echo.Context) error {
		keyword := c.FormValue("search")

		if keyword == "" {
			page.Data.SearchResults = nil
		} else {
			keyword = strings.ToLower(keyword)

			page.Data.SearchResults = make([]Word, 0, len(page.Data.Words))

			searchQry := fmt.Sprintf("select * from words where original like '%%%s%%' or meaning like '%%%s%%'", keyword, keyword)
			fmt.Println(searchQry)

			rows, err := db.Query(searchQry)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			var defn Word
			for rows.Next() {
				err := rows.Scan(&defn.Id, &defn.Original, &defn.Translation)
				if err != nil {
					log.Fatal(err)
				}
				//log.Println(defn.Id, defn.Original)
				page.Data.SearchResults = append(page.Data.SearchResults, Word{Id: defn.Id, Original: defn.Original, Translation: defn.Translation})
			}

			if err := rows.Err(); err != nil {
				log.Fatal(err)
			}
		}

		return c.Render(200, "search-results", page.Data)
	})

	e.DELETE("/words/:id", func(c echo.Context) error {
		time.Sleep(500 * time.Millisecond)
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)

		if err != nil {
			return c.String(400, "Invalid id")
		}

		index := page.Data.indexOf(id)
		if index == -1 {
			return c.String(404, "Word not found")
		}

		deleteQry := fmt.Sprintf("delete from words where word_id = %d", id)
		_, err = db.Exec(deleteQry)
		if err != nil {
			fmt.Printf("couldn't delete word: %s", err)
		}

		page.Data.Words = append(page.Data.Words[:index], page.Data.Words[index+1:]...)

		return c.NoContent(200)
	})

	e.Logger.Fatal(e.Start(":42069"))
}
