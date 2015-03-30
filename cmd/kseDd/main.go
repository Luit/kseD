package main

import (
	"encoding/json"
	"expvar"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/Luit/kseD"
)

var (
	d        *kseD.Device
	user     int // index in heights       (from config.go)
	position int // index in heights[user]

	signals = make(chan os.Signal, 4)
)

func main() {
	var err error
	d, err = kseD.New("/dev/ttyAMA0")
	if err != nil {
		log.Fatal(err)
	}
	log.Print("kseDd connected to kseD")

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM) // TODO: more signals?
	go signalHandler()
	go configLoop()
	go cardLoop()
	go http.ListenAndServe(":8080", nil) // TODO: random port & print port
	runtime.Goexit()
}

func signalHandler() {
	// sig := <-signals
	<-signals
	teardown(true)
}

func teardown(clean bool) {
	err := d.Close()
	if err != nil {
		log.Fatal(err)
	}
	if clean {
		log.Print("kseDd terminated cleanly")
		os.Exit(0)
	}
	log.Print("kseDd terminated")
	os.Exit(1)
}

func cardLoop() {
	for {
		uid, err := d.ReadCard()
		if err != nil {
			log.Printf("failed to read card: %s", err.Error())
			teardown(false)
		}
		var (
			newUser int
			ok      bool
		)
		var v []byte
		switch len(uid) {
		case 4:
			newUser, ok = uid4[bytes4(uid)]
			v, _ = json.Marshal(bytes4(uid))
		case 7:
			newUser, ok = uid7[bytes7(uid)]
			v, _ = json.Marshal(bytes7(uid))
		case 10:
			newUser, ok = uid10[bytes10(uid)]
			v, _ = json.Marshal(bytes10(uid))
		default:
			log.Printf("Got unexpected uid length %d (%v)", len(uid), uid)
			continue
		}
		if !ok {
			if !ok {
				log.Printf("Card not recognized: %s", v)
			}
			continue
		}
		log.Printf("Card recorgnized for user %d", user)
		if newUser == user {
			position = (position + 1) % len(heights[user])
			d.Move(heights[user][position])
		} else {
			user = newUser
			position = position % len(heights[user])
			d.Move(heights[user][position])
		}
		log.Printf("Moving to position %d; height %d (%dmm)",
			position, heights[user][position],
			kseD.ToMilli(heights[user][position]))
	}
}

var (
	expConfSha = expvar.NewString("config_sha")
)

func configLoop() {
	sha := ""
	for {
		newSha, err := loadConfig(sha)
		if err != nil {
			log.Printf("failed to load config: %s", err.Error())
		} else {
			if newSha != "" && newSha != sha {
				sha = newSha
				expConfSha.Set(sha)
				log.Printf("Loaded configuration %s\n", sha)
			}
		}
		if i, _ := strconv.ParseInt(rateLimitRemaining.String(), 10, 0); i < 60 {
			<-time.Tick(10 * time.Minute)
		} else {
			<-time.Tick(10 * time.Second)
		}
	}
}
