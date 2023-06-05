package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"strings"
	"fmt"
	"github.com/urfave/cli/v2"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/static"
	"regexp"
	"net/url"
	"net/http"
	"encoding/json"
	"strconv"
)

type dbItem struct {
	Magnet      string            `json:"magnet"`
	Title       string            `json:"title"`
	Datetime    string            `json:"dt"`
	Categories  string            `json:"cat"`
	Size        sql.NullString    `json:"size"`
	IMDB        sql.NullString    `json:"imdb"`
}

func frontendFolderExists() (bool, error) {
	_, err := os.Stat("frontend")
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
	return false, err
}

func ConnectDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./db.sqlite")
	if err != nil {
		return nil, err
	}
	return db, nil
}
func main() {
	var port string
	var serveFrontend bool = false 
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "port",
				Usage: "Specify the port to run the API on",
				Destination: &port,
				Required: true,
			},
			&cli.BoolFlag{
				Name: "serve-frontend",
				Value: false,
				Usage: "Serve frontend along the API",
				Destination: &serveFrontend,
			},
		},
		Action: func(c *cli.Context) error {
			return nil
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
	rarbgDB, err := ConnectDatabase()
	if err != nil {
		log.Fatal(err)
	}
	r := gin.Default()
	r.Use(cors.Default())
	frontendExists, _ := frontendFolderExists()
	if serveFrontend && frontendExists {
		r.Use(static.Serve("/", static.LocalFile("frontend", false)))
	} else {
		r.GET("/", func(c *gin.Context) {
			c.String(200, "ShadowBG")
		})
	}
	
	router := r.Group("/api")
	{		
		router.GET("", mainPage(rarbgDB))
		router.GET("search", searchResults(rarbgDB))
	}

	r.Run(":"+string(port))
}

func mainPage(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int
		err := db.QueryRow("SELECT COUNT(*) as count FROM items").Scan(&count)
		if err != nil {
			c.JSON(404, gin.H{"error": err})
		} else {
			c.Status(200)
			writeContentType(c.Writer, []string{"application/json; charset=utf-8"})
			enc := json.NewEncoder(c.Writer)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(gin.H{"message": "We have "+strconv.Itoa(count)+" records. Please use /api/search?q=<Search query>"}); err != nil {
				log.Fatal(err)
			}
		}
		
	}
}

func writeContentType(w http.ResponseWriter, value []string) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = value
	}
}

func searchResults(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		page, _ := strconv.Atoi(c.Query("page"))
		offset := 0
		if page > 0 {
			offset = (page-1)*30
		}
		if len(query) == 0 || query == "\"\"" {
			c.JSON(404, gin.H{"query": query, "results": "", "error": "Invalid query"})
			return
		}
		items, err := Search(query, db, offset)
		if err != nil {
			c.JSON(404, gin.H{"query": query, "results": "", "error": err})
		} else {
			c.Status(200)
			writeContentType(c.Writer, []string{"application/json; charset=utf-8"})
			enc := json.NewEncoder(c.Writer)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(gin.H{"query": query, "results": items}); err != nil {
				log.Fatal(err)
			}
		}
		
		
	}
}

func Search(query string, DB *sql.DB, offset int) ([]dbItem, error) {
	query = strings.Replace(query, "\"", "", -1)
	imdbMatchReg, _ := regexp.Compile("tt([0-9]+)")
	imdbMatch := imdbMatchReg.FindString(query)
	items := make([]dbItem, 0)
	if imdbMatch != query {
		modifiedQuery := "SELECT hash,title,dt,cat,size,imdb FROM items WHERE title LIKE "+ `"%`+strings.Replace(query, " ", "%", -1)+`%" ORDER BY dt DESC LIMIT 30 OFFSET `+strconv.Itoa(offset)
		rows, err := DB.Query(modifiedQuery)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			_DBItem := dbItem{}
			var hash string
			err = rows.Scan(&hash, &_DBItem.Title, &_DBItem.Datetime, &_DBItem.Categories, &_DBItem.Size, &_DBItem.IMDB)
			_DBItem.Magnet = "magnet:?xt=urn:btih:"+strings.ToLower(hash)+"&dn="+url.QueryEscape(_DBItem.Title)+"&tr=http%3A%2F%2Ftracker.trackerfix.com%3A80%2Fannounce&tr=udp%3A%2F%2F9.rarbg.me%3A2740&tr=udp%3A%2F%2F9.rarbg.to%3A2760&tr=udp%3A%2F%2Ftracker.tallpenguin.org%3A15750&tr=udp%3A%2F%2Ftracker.thinelephant.org%3A12770"
			if err != nil {
				fmt.Println(err)
				return nil, err
			}

			items = append(items, _DBItem)
		}
		err = rows.Err()
		if err != nil {
			return nil, err
		}
	}
	if imdbMatch != "" {
		imdbQuery := "SELECT hash,title,dt,cat,size,imdb FROM items WHERE imdb="+`"`+imdbMatch+`" ORDER BY dt DESC LIMIT 30 OFFSET `+strconv.Itoa(offset)
		imdbRows, err := DB.Query(imdbQuery)
		if err != nil {
			return nil, err
		}
		defer imdbRows.Close()
		for imdbRows.Next() {
			_DBItem := dbItem{}
			var hash string
			err = imdbRows.Scan(&hash, &_DBItem.Title, &_DBItem.Datetime, &_DBItem.Categories, &_DBItem.Size, &_DBItem.IMDB)
			_DBItem.Magnet = "magnet:?xt=urn:btih:"+strings.ToLower(hash)+"&dn="+url.QueryEscape(_DBItem.Title)+"&tr=http%3A%2F%2Ftracker.trackerfix.com%3A80%2Fannounce&tr=udp%3A%2F%2F9.rarbg.me%3A2740&tr=udp%3A%2F%2F9.rarbg.to%3A2760&tr=udp%3A%2F%2Ftracker.tallpenguin.org%3A15750&tr=udp%3A%2F%2Ftracker.thinelephant.org%3A12770"
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			items = append(items, _DBItem)
		}
		err = imdbRows.Err()
		if err != nil {
			return nil, err
		}
	}
	return items, nil
}