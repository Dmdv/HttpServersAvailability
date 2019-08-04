package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"database/sql"

	_ "github.com/lib/pq"
	"github.com/olebedev/config"

	"io/ioutil"
)

type Page struct {
	Name     string
	DBStatus bool
}

type Pages []Page

type ServerStatus struct {
	Available string
	Url       string
	Time      string
}

type ServerStatuses []ServerStatus

type Settings struct {
	host     string
	port     string
	database string
	user     string
	pass     string
}

func main() {

	// var pg Pages

	// pg = append(pg, Page{"First Page", true})
	// pg = append(pg, Page{"Second Page", true})

	// pagesJson, err := json.Marshal(pg)
	// if err != nil {
	// 	log.Fatal("Cannot encode to JSON ", err)
	// }

	templates := template.Must(template.ParseFiles("index.html"))

	settings := readSettings("settings.yaml")

	connStr := fmt.Sprintf("postgres://%s:%s@localhost:%s/status?sslmode=disable", settings.user, settings.pass, settings.port)

	db := createConnection(connStr)
	defer db.Close()

	fmt.Println("Server started OK")

	dir := http.Dir("./assets/")
	handler := http.StripPrefix("/assets/", http.FileServer(dir))
	http.Handle("/assets/", handler)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Serving /...")

		p := Page{Name: "Gopher"}

		if name := r.FormValue("name"); name != "" {
			p.Name = name
		}
		p.DBStatus = db.Ping() == nil

		if err := templates.ExecuteTemplate(w, "index.html", p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Serving refresh...")

		if r.Method == http.MethodPost {
			http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		}

		w.Header().Set("Content-Type", "application/json")

		var results ServerStatuses
		var err error

		if results, err = refresh(db); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		encoder := json.NewEncoder(w)

		if err := encoder.Encode(results); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	fmt.Println(http.ListenAndServe(":8080", nil))
}

func readSettings(location string) *Settings {
	file, err := ioutil.ReadFile(location)
	if err != nil {
		panic(err)
	}

	yamlString := string(file)

	cfg, _ := config.ParseYaml(yamlString)

	var host string
	var port string
	var database string
	var user string
	var pass string

	host, err = cfg.String("host")
	if err != nil {
		panic("Failed to read 'host' from config")
	}
	port, err = cfg.String("port")
	if err != nil {
		panic("Failed to read 'port' from config")
	}
	database, err = cfg.String("database")
	if err != nil {
		panic("Failed to read 'database' from config")
	}
	user, err = cfg.String("user")
	if err != nil {
		panic("Failed to read 'user' from config")
	}
	pass, err = cfg.String("pass")
	if err != nil {
		panic("Failed to read 'pass' from config")
	}

	return &Settings{
		host:     host,
		port:     port,
		database: database,
		user:     user,
		pass:     pass,
	}
}

func createConnection(connStr string) *sql.DB {

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Println("Connection error or 'Status' database doesn't exist")
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		fmt.Println("Connection error or 'Status' database doesn't exist")
		panic(err)
	}
	fmt.Println("Ping OK")

	ctx, _ := context.WithTimeout(context.Background(), time.Nanosecond)
	err = db.PingContext(ctx)
	if err != nil {
		fmt.Println("Connection error or 'Status' database doesn't exist")
		fmt.Println("Error: " + err.Error())
	}

	conn, err := db.Conn(context.Background())
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	err = conn.PingContext(context.Background())
	if err != nil {
		fmt.Println("Connection error or 'Status' database doesn't exist")
		panic(err)
	}

	fmt.Println("Connection Ping OK.")
	fmt.Println("Status database exists")

	return db
}

func refresh(db *sql.DB) (ServerStatuses, error) {

	rows, err := db.Query("select url, available, time from servers where (now() - time) < interval '5 minutes';")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var statuses ServerStatuses

	for rows.Next() {

		var time string
		var url string
		var available string

		err := rows.Scan(&url, &available, &time)
		if err != nil {
			log.Fatal(err)
			return statuses, err
		}

		status := ServerStatus{
			Time:      time,
			Available: available,
			Url:       url,
		}

		statuses = append(statuses, status)
	}

	err = rows.Err()
	if err != nil {
		log.Fatal(err)
	}

	return statuses, err
}
