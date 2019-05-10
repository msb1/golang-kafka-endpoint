package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// constants for Kafka Broker setup
const (
	sessionTimeoutMs int           = 6000
	autoOffsetReset  string        = "earliest"
	webSocketTimeout time.Duration = 20.0 * time.Second
)

// Parameter is a struct that links to websocket and Kafka broker
type Parameter struct {
	lastResponse    time.Time
	EpdSimName      string
	BootstrapServer string
	ProducerTopic   []string
	ConsumerTopic   []string
	GroupID         string
	WebMessage      chan string
	PTotal          int
	CTotal          int
}

var mutex = &sync.Mutex{}

// map of kafka broker configs for each connection
var broker = make(map[string]*Parameter)

// map of websocket connections
var connection = make(map[string]*websocket.Conn)

// simulator configuration
var config Config

// websocket upgrader http to ws
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// parse main html app page so it is available for ParseForm()
var tmpl = template.Must(template.ParseFiles("templates/epdsim.html"))

// root hander for "/"
func rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, "templates/home.html")
}

// handler for "/epd"
func epdHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/epd" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// parse form data from home.html to setup Kafka broker
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Bad form data", http.StatusNotFound)
	}

	// parse producer and consumer topics and make into arrays (topics must be separated by commas)
	ptopic := strings.Split(r.PostFormValue("producerTopic"), ",")
	ctopic := strings.Split(r.PostFormValue("consumerTopic"), ",")

	for i := range ptopic {
		ptopic[i] = strings.TrimSpace(ptopic[i])
	}
	for i := range ctopic {
		ctopic[i] = strings.TrimSpace(ctopic[i])
	}

	param := &Parameter{
		EpdSimName:      r.PostFormValue("epdSimName"),
		BootstrapServer: r.PostFormValue("bootstrapServer"),
		ProducerTopic:   ptopic,
		ConsumerTopic:   ctopic,
		GroupID:         r.PostFormValue("groupId"),
		WebMessage:      make(chan string),
		PTotal:          0,
		CTotal:          0,
	}
	broker[param.EpdSimName] = param

	fmt.Printf("%s at Bootstrap Server: %s with GroupId: %s\nProducer Topic: %s, Consumer Topic: %s",
		param.EpdSimName, param.GroupID, param.BootstrapServer, param.ProducerTopic, param.ConsumerTopic)

	// execute parsed template so that input form values are displayed on the main app page
	err = tmpl.Execute(w, param)
	if err != nil {
		log.Fatal(err)
	}
}

// handler for "/ws" websocket (non-secured)
func wsHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println(r.Header)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Upgrader error...")
		log.Fatal(err)
	}
	// Read message from browser
	conn.SetReadDeadline(time.Now().Add(webSocketTimeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		fmt.Println("conn read error...")
		return
	}
	// Print the message to the console
	epdSimName := string(msg)
	fmt.Printf("%s sent: %s...\n", r.Header["Origin"], epdSimName)
	connection[epdSimName] = conn

	// start go routines to listen and send on websocket
	go pingWebsocket(connection[epdSimName], broker[epdSimName])
	go brokerConsumer(connection[epdSimName], broker[epdSimName])
	go listenToWebSocket(connection[epdSimName], broker[epdSimName])
	go sendRecords(connection[epdSimName], broker[epdSimName])
}

// goroutine to listen to websocket for operational commands (and ping/pong)
func listenToWebSocket(conn *websocket.Conn, p *Parameter) {

	for {
		// Read message from browser
		conn.SetReadDeadline(time.Now().Add(webSocketTimeout))
		_, evt, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("conn read error...")
			return
		}

		// webMessage channel is used to signal other parts of the progam to start/stop/close
		msg := string(evt)
		// check message type and take appropriate action
		if msg == "pong" {
			p.lastResponse = time.Now()
			fmt.Println("Pong message received: " + string(msg))
			p.WebMessage <- "pong"
		} else {
			fmt.Printf("%s message sent from epdSim web page...\n", string(msg))
			p.WebMessage <- string(msg)
			if string(msg) == "close" {
				return
			}
		}

		// stop goroutine if timeout exceeded
		if time.Now().Sub(p.lastResponse) > webSocketTimeout {
			p.WebMessage <- "close"
			return
		}
	}
}

// functional KafkaConsumer goroutine (as channel Consumer is deprecated)
func brokerConsumer(conn *websocket.Conn, p *Parameter) {
	// initialize functional KafkaConsumer
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":     p.BootstrapServer,
		"broker.address.family": "v4",
		"group.id":              p.GroupID,
		"session.timeout.ms":    sessionTimeoutMs,
		"auto.offset.reset":     autoOffsetReset})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create consumer: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created Consumer %v\n", consumer)

	err = consumer.SubscribeTopics(p.ConsumerTopic, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to subscribe to consumer topic: %s\n", err)
		os.Exit(1)
	}

	run := true
	for run == true {
		select {
		case msg := <-p.WebMessage:
			if msg == "close" {
				fmt.Printf("Close message received -- terminating consumer: %s\n", p.EpdSimName)
				run = false
			}
		default:
			msg, err := consumer.ReadMessage(time.Second)
			if err == nil {
				// Process received messages - for now send to web page and/or print to console
				fmt.Printf("Message on %s: %s\n", msg.TopicPartition, string(msg.Value))
				jsonString := fmt.Sprintf("%s,,,%s", "consumer", msg.Value)
				mutex.Lock()
				conn.SetWriteDeadline(time.Now().Add(webSocketTimeout))
				err = conn.WriteMessage(websocket.TextMessage, []byte(jsonString))
				mutex.Unlock()

				if err != nil {
					log.Printf("Websocket %s error: %s \n", p.EpdSimName, err)
					conn.Close()
					delete(connection, p.EpdSimName)
					return
				}

				p.CTotal++
				sendStatusMessage(conn, p)

			} else {
				// The client will automatically try to recover from all errors.
				fmt.Printf("Consumer error: %v (%v)\n", err, msg)
				continue
			}
		}
	}
	fmt.Printf("Closing consumer\n")
	consumer.Close()

}

// goroutine to send data records to channel KafkaProducer and websocket (for display on web page)
func sendRecords(conn *websocket.Conn, p *Parameter) {

	// initialize channel KafkaProducer
	producer, err := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": p.BootstrapServer})
	if err != nil {
		fmt.Printf("Failed to create producer: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created Producer %v\n", producer)

	//doneChan := make(chan bool)

	runflag := false
	for {
		if runflag {
			// get data record
			js := makeSimulatedRecord(config, p.ProducerTopic[0])
			// send data record to kafka broker
			topic := p.ProducerTopic[0]
			producer.ProduceChannel() <- &kafka.Message{TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny}, Value: []byte(js)}

			// listen for producer delivery event on producer channel
			evt := <-producer.Events()
			switch m := evt.(type) {
			case *kafka.Message:
				if m.TopicPartition.Error != nil {
					fmt.Printf("Delivery failed: %v\n", m.TopicPartition.Error)
				} else {
					fmt.Printf("Delivered message to topic %s [%d] at offset %v\n", *m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
					jsonString := fmt.Sprintf("%s,,,%s", "producer", js)
					fmt.Println(jsonString)
					fmt.Println(" ")
					// write data record to websocket
					mutex.Lock()
					conn.SetWriteDeadline(time.Now().Add(webSocketTimeout))
					err := conn.WriteMessage(websocket.TextMessage, []byte(jsonString))
					mutex.Unlock()

					if err != nil {
						log.Printf("Websocket %s error: %s \n", p.EpdSimName, err)
						conn.Close()
						delete(connection, p.EpdSimName)

						return
					}
					p.PTotal++
					sendStatusMessage(conn, p)
				}
			default:
				fmt.Printf("Ignored event: %s\n", m)
			}
		}

		// listen for webMessage to start/stop/close application
		msg := <-p.WebMessage
		if msg == "pong" {
			continue
		} else if msg == "start" {
			runflag = true
		} else if msg == "stop" {
			runflag = false
		} else if msg == "close" {
			log.Printf("Closing Websocket %s... \n", p.EpdSimName)
			conn.Close()
			delete(connection, p.EpdSimName)
			break
		}
		// add some randomness to when messages are generated and sent
		randomDelay := time.Duration(rand.Intn(int(webSocketTimeout / 8)))
		time.Sleep(randomDelay)
	}
}

func pingWebsocket(conn *websocket.Conn, p *Parameter) {

	// last response is updated everytime a pong message is received
	p.lastResponse = time.Now()

	// embedded goroutine to send ping messages
	go func() {
		for {
			jsonString := fmt.Sprintf("%s,,,%s", "control", "ping")
			mutex.Lock()
			conn.SetWriteDeadline(time.Now().Add(webSocketTimeout))
			err := conn.WriteMessage(websocket.TextMessage, []byte(jsonString))
			mutex.Unlock()
			fmt.Println("Ping Message - keepalive - sent...")
			if err != nil {
				log.Fatal(err)
				return
			}
			time.Sleep(webSocketTimeout / 2)
			if time.Now().Sub(p.lastResponse) > webSocketTimeout {
				fmt.Printf("Timeout on websocket connection: %s \n", p.EpdSimName)
				return
			}
		}
	}()
}

func sendStatusMessage(conn *websocket.Conn, p *Parameter) {

	status := fmt.Sprintf("%s,,,EPD Simulator Totals --> Produced: %d   Consumed: %d", "status", p.PTotal, p.CTotal)
	fmt.Println(status)
	mutex.Lock()
	conn.SetWriteDeadline(time.Now().Add(webSocketTimeout))
	err := conn.WriteMessage(websocket.TextMessage, []byte(status))
	mutex.Unlock()
	if err != nil {
		log.Printf("Websocket %s error: %s \n", p.EpdSimName, err)
		conn.Close()
		delete(connection, p.EpdSimName)
		return
	}
}

// main function for application
func main() {
	// init random number generator
	rand.Seed(time.Now().UTC().UnixNano())
	// read in configuration data from fil
	config = readConfigData("epd.conf")

	// open gorilla mux router
	router := mux.NewRouter()
	router.HandleFunc("/", rootHandler)
	router.HandleFunc("/epd", epdHandler)
	router.HandleFunc("/ws", wsHandler)
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	log.Fatal(http.ListenAndServe(":8000", router))
	fmt.Println("Program terminated...")
}
