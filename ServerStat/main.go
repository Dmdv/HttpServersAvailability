package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/olebedev/config"

	"github.com/robfig/cron"
)

type Settings struct {
	host           string
	port           string
	database       string
	user           string
	pass           string
	httptimeout_ms int
	servers        []string
}

type ServerStatus struct {
	available bool
	url       string
	time      time.Time
}

func addRecord(status *ServerStatus, wg *sync.WaitGroup, db *sql.DB) {
	defer wg.Done()
	var lastInsertId int
	err := db.QueryRow("insert into servers(URL, AVAILABLE, TIME) values($1, $2, $3) returning id;",
		status.url, status.available, status.time).Scan(&lastInsertId)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Status %s => %t\n", status.url, status.available)
}

func monitorServerStatus(source chan *ServerStatus, wg *sync.WaitGroup, db *sql.DB) {
	for {
		select {
		case status := <-source:
			go addRecord(status, wg, db)
			continue
		}
	}
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
	var httptimeout_ms int
	var user string
	var pass string
	var servers []interface{}

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
	httptimeout_ms = cfg.UInt("httptimeout_ms")
	if err != nil {
		panic("Failed to read 'httptimeout_msost' from config")
	}
	servers, err = cfg.List("servers")
	if err != nil {
		panic("Failed to read 'servers' from config")
	}
	user, err = cfg.String("user")
	if err != nil {
		panic("Failed to read 'user' from config")
	}
	pass, err = cfg.String("pass")
	if err != nil {
		panic("Failed to read 'pass' from config")
	}

	var serverList []string = make([]string, len(servers))

	for idx, item := range servers {
		serverList[idx] = item.(string)
	}

	return &Settings{
		host:           host,
		port:           port,
		database:       database,
		servers:        serverList,
		httptimeout_ms: httptimeout_ms,
		user:           user,
		pass:           pass,
	}
}

func httpStatusChecker(url string, timeout_ms int, target chan *ServerStatus) {

	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}

	fmt.Println("Check status: " + url)

	timeout := time.Duration(time.Duration(timeout_ms) * time.Second)

	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)

	status := &ServerStatus{
		time:      time.Now(),
		url:       url,
		available: err == nil && resp != nil && resp.StatusCode < 400,
	}

	if resp != nil {
		defer resp.Body.Close()
	}

	target <- status
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

func main() {

	c := cron.New()

	c.AddFunc("*/60 * * * *", workerUnit)

	go c.Start()
	defer c.Stop()

	sig := make(chan os.Signal)
	signal.Notify(sig, os.Interrupt, os.Kill)
	<-sig
}

func workerUnit() {

	settings := readSettings("settings.yaml")

	connStr := fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable", settings.user, settings.pass, settings.port, settings.database)

	db := createConnection(connStr)
	defer db.Close()

	var wg sync.WaitGroup

	iter := len(settings.servers)

	wg.Add(iter)
	eventSource := make(chan *ServerStatus, iter)

	go monitorServerStatus(eventSource, &wg, db)

	for _, server := range settings.servers {
		fmt.Println("Run " + server)
		go httpStatusChecker(server, settings.httptimeout_ms, eventSource)
	}

	wg.Wait()

	fmt.Println("Completed")
}
