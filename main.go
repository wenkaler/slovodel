package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3"
)

var (
	//GODMOD ...
	GODMOD    = false
	out       = make(chan string)
	wg        = sync.WaitGroup{}
	connetion *sql.DB // coccect to database sqlite3 - filename slave.db
	host      = flag.String("h", "localhost:9090", "host, send all word from the chanel in this host.")
)

func main() {
	var (
		file = flag.String("f", "", "file path, use when need loads some data in the base.")
		word = flag.String("w", "", "word, use when you wanna find subwords, who contain in this word.")
		god  = flag.Bool("g", false, "god mode, you don't need to do anything.")
	)
	flag.Parse()

	if *file != "" {
		f, err := os.Open(*file)
		if err != nil {
			log.Fatalf("Fail open file(%v): %v", file, err)
		}
		defer f.Close()
		var scaner = bufio.NewScanner(f)
		for scaner.Scan() {
			var row = scaner.Text()
			err := insert(row)
			if err != nil {
				log.Printf("Failed insert row(%v): %v", row, err)
			}
		}
	} else if *word != "" {
		go send(out)
		findSubWords(*word)

	} else if *god {
		GODMOD = true
		go send(out)
		for {
			var (
				reply     = make([]byte, 512)
				r         = regexp.MustCompile("([аА-яЯ]{1,100})")
				conn, err = net.Dial("tcp", *host)
			)
			if err != nil {
				log.Fatalf("connection failed to (%v): %v", host, err)
			}
			fmt.Fprintln(conn, "msg WordsGame-bot /play")
			time.Sleep(5 * time.Second)
			fmt.Fprintln(conn, "history WordsGame-bot 1")
			time.Sleep(2 * time.Second)
			_, err = conn.Read(reply)
			if err != nil {
				log.Fatalf("failed read connection %v", err)
			}
			word := r.FindAllString(string(reply), 1)
			if len(word) <= 0 {
				log.Fatalf("failed answer is not contain string: %s", reply)
			}
			findSubWords(word[0])
			time.Sleep(5 * time.Minute)
		}
	} else {
		flag.Usage()
	}
}

func findSubWords(word string) {
	list := decay(word)
	for length := 2; length <= utf8.RuneCountInString(word); length++ {
		wg.Add(1)
		go func(out chan<- string, length int) {
			search(out, list, length)
			wg.Done()
			fmt.Println("Done: ", length)
		}(out, length)
	}
	wg.Wait()
	fmt.Println("search done")
	if !GODMOD {
		close(out)
	}
}

func send(in <-chan string) {
	conn, err := net.Dial("tcp", *host)
	if err != nil {
		log.Fatalf("connection failed to (%v): %v", host, err)
	}
	for word := range in {
		fmt.Fprintf(conn, "msg WordsGame-bot %v\n", word)
		time.Sleep(5 * time.Second)
	}
}

func selects(length int) (wordList []string, err error) {
	rows, err := connetion.Query("SELECT word FROM words WHERE length=?", length)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var word string
		err = rows.Scan(&word)
		if err != nil {
			return
		}
		wordList = append(wordList, word)
	}
	return
}

func search(out chan<- string, wordRuneList map[rune]int, length int) {

	wordList, err := selects(length)
	if err != nil {
		log.Printf("fail length %v, error: %v", length, err)
	}

	for _, word := range wordList {
		var (
			wordCopyList = make(map[rune]int)
			contain      = true
		)
		for k, v := range wordRuneList {
			wordCopyList[k] = v
		}
		for _, r := range word {
			if _, ok := wordCopyList[r]; ok && wordCopyList[r] > 0 {
				wordCopyList[r]--
			} else {
				contain = false
				break
			}
		}
		if contain {
			out <- word
		}
	}
}

func init() {
	var err error
	connetion, err = sql.Open("sqlite3", "./slave.db")
	if err != nil {
		log.Fatalf("Failed connection: %v", err)
	}
	_, err = connetion.Exec(`CREATE TABLE IF NOT EXISTS words (word VARCHAR(225) UNIQUE NOT NULL, length INTEGER NOT NULL);`)
	if err != nil {
		log.Fatalf("Failed create database table words: %v", err)
	}
}

func insert(word string) error {
	_, err := connetion.Exec("INSERT INTO words (word,length) VALUES(?,?)", word, utf8.RuneCountInString(word))
	if err != nil && err.Error() != "UNIQUE constraint failed: words.word" {
		return err
	}
	return nil
}

func decay(word string) map[rune]int {
	var m = make(map[rune]int)
	for _, char := range word {
		m[char]++
	}
	return m
}
