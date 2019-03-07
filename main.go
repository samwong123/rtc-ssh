package main

import (
	"net/url"
	"os"
	"fmt"
	"log"
	"runtime"
	"path/filepath"
	"os/signal"
	"time"
//	"github.com/pions/webrtc"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
ini "github.com/pierrec/go-ini"
)



const usage = `Usage: server [commands]
commands:
	newkey     new generate key uuid of connection
	getkey     get key uuid
`
type Config struct {
	Uuid     string        `ini:"uuid,identify"`
	Host     string        `ini:"host,ssh"`
	Port     int           `ini:"port,ssh"`
}

type Json struct {
	Type string
	Sdp string
	Error string
}



func connect(query string) *websocket.Conn {
    var u url.URL
	var conn *websocket.Conn
	var err error
	
	u = url.URL{Scheme: "wss", Host: "www.sqs.io", Path: "/signal", RawQuery: query}
    for {
        conn, _, err = websocket.DefaultDialer.Dial(u.String(), nil)
        if err != nil {
            log.Println(err)
            time.Sleep(30 * time.Second)
            continue
        }
       break
    }

	lastResponse := time.Now()
    conn.SetPongHandler(func(msg string) error {
       lastResponse = time.Now()
	   return nil
	})
		
	go func() {
     for {
        err := conn.WriteMessage(websocket.PingMessage, nil)
        if err != nil {
            return 
        }   
        time.Sleep((120 * time.Second)/2)
        if(time.Now().Sub(lastResponse) > (120 * time.Second)) {
            log.Println("signaling server close connection")
			conn.Close()
		    return
        }
    }
  }()
  
    return conn
}


func main() {
	_, callerFile, _, _ := runtime.Caller(0)
	executablePath := filepath.Dir(callerFile)
	log.SetFlags(0)
	
	
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	
	file, err := os.Open(executablePath + "/config.ini")
	check(err)
	defer file.Close()
	var conf Config
	err = ini.Decode(file, &conf)
	check(err)
	
	cmd := ""
		
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	
	switch cmd {
		case "newkey":
			conf.Uuid = uuid.New().String()
			file, err := os.Create("config.ini")
			check(err)
			defer file.Close()
			err = ini.Encode(file, &conf)
			check(err)
			fmt.Println("uuid:", conf.Uuid)	
		case "getkey":
			fmt.Println("uuid:", conf.Uuid)
		case "help":
			fmt.Println(usage)
			os.Exit(0)
	}
		
	
	done := make(chan struct{})
	c_hub := make(chan struct{})
	go func() {
		for {
			hub(c_hub, conf)
			time.Sleep(30 * time.Second)
			log.Println("signaling server reconnect")
		}
	}()	
	
	
	for {
		select {
		case <-done:
			return
		case <-interrupt:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			fmt.Println("interrupt")
			close(c_hub)
			select {
				case <-done:
				case <-time.After(time.Second):
			}
			return
			
		}
		
	}

}


func hub(c_hub <-chan struct{}, conf Config) {
	
//	done := make(chan struct{})
	query := "localUser=" + conf.Uuid
	conn := connect(query)
	defer conn.Close()
	go func(){
		for{
			select {
			case <-c_hub:
				err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Println("write close:", err)
				}
				return
			}
		}
	}()
	message := Json{}
	for {
		err := conn.ReadJSON(&message)
		if err == nil {
			err = interpreter(conn, message, conf)
			if err != nil {log.Println(err)}
		}else {
			_,ok:= err.(*websocket.CloseError) 
			if  ok == false {
				log.Println("websocket", err)
			}
			break
		}
	}
	
	
}

func check(e error) {
    if e != nil {
        log.Println(e)
    }
}