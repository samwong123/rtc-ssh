package main

import (
	"net/url"
	"os"
	"fmt"
	"log"
	"os/signal"
	"time"
	"path/filepath"
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
	Type string   `json:"type"`
	Sdp  string   `json:"sdp"`	
	Error string   `json:"error"`
}



func connect(query string) *websocket.Conn {
	var u url.URL
	var conn *websocket.Conn
	var err error
	
	u = url.URL{Scheme: "wss", Host: "sqs.io", Path: "/signal", RawQuery: query}
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
	log.SetFlags(0)
		
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	
	var executablePath string
	ex, err := os.Executable()
    	check(err)
	executablePath = filepath.Dir(ex)
	cmd := ""
		
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	
	var conf Config
	
	getConf := func(){
		file, err := os.Open(executablePath + "/config.ini") 
		if os.IsNotExist(err) && cmd != "newkey"{
			log.Println("File config.ini not found, using option 'newkey'")
			os.Exit(0)
		} 
		defer file.Close()	
		err = ini.Decode(file, &conf)
		check(err)
	}
	
	switch cmd {
		case "newkey":
			conf.Uuid = uuid.New().String()
			conf.Host = defaultHost
			conf.Port = defaultPort
			file, err := os.Create(executablePath + "/config.ini")
			check(err)
			defer file.Close()
			err = ini.Encode(file, &conf)
			check(err)
			fmt.Println("uuid:", conf.Uuid)	
		case "getkey":
			getConf()
			fmt.Println("uuid:", conf.Uuid)
		case "help":
			fmt.Println(usage)
			os.Exit(0)
		default:
			getConf()
	}
		
	
	done := make(chan struct{})
	c_hub := make(chan struct{})
	
	go func() {
		for {
			hub(c_hub, conf)
			time.Sleep(30 * time.Second)
			log.Println("reconnect with the signaling server")
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
		if err != nil {
			_,ok:= err.(*websocket.CloseError) 
			if !ok {log.Println("websocket", err)}
			break
		} 
		err = interpreter(conn, message, conf)
		if err != nil {log.Println(err)}
	}
}

func check(e error) {
    if e != nil {
        log.Println(e)
    }
}
