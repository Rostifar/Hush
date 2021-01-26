package main

import (
	"encoding/json"
	"fmt"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/gen2brain/beeep"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"
)

// App data constants
const (
	APP_NAME = "Hush"
	APP_ICON_PATH = "background.jpg"
)

type Config struct {
	Email string `json:"email"`
	AppID string `json:"app_id"`
	ValidEmails []string `json:"valid_emails"`
}

type MailboxCache struct {
	localMessages []LocalMessage
	mux sync.Mutex
}

type Date struct {
	Day int `json:"day"`
	Month int `json:"month"`
	Year int `json:"year"`
}

type Time struct {
	Hour int `json:"hour"`
	Minute int `json:"minute"`
}

type TimeInterval struct {
	start, end Time
}

func initTimeInteral(start, end *Time) {
}

func (tInt *TimeInterval) inTimeInterval(t *Time) bool {
	return tInt.start.Hour > t.Hour ||
			t.Hour > tInt.end.Hour  ||
			(t.Hour == tInt.start.Hour &&
				t.Minute < tInt.start.Minute) ||
			(t.Hour == tInt.end.Hour &&
				t.Minute > tInt.end.Minute)
}

type LocalEnvelope struct {
	Authors []string `json:"author"`
	SenderAddresses []string `json:"sender_address"`
	ReceivedDate Date `json:"date"`
	ReceivedTime Time `json:"time"`
	Subject string `json:"subject"`
	MessageID string `json:"message_id"`
	CC []string `json:"cc"`
	BCC []string `json:""`
}


type LocalMessage struct {
	LocalEnvelope LocalEnvelope `json:"envelope"`
//	LocalBody LocalBody `json:"body"`
}


func imapMessageToLocalMessage(msg *imap.Message) *LocalMessage {
	lEnv := LocalEnvelope{}

	for _, address := range msg.Envelope.Sender {
		lEnv.Authors = append(lEnv.Authors, address.PersonalName)
		lEnv.SenderAddresses = append(lEnv.SenderAddresses, address.Address())
	}

	year, month, day := msg.Envelope.Date.Date()
	lEnv.ReceivedDate = Date{Day: day, Month: int(month), Year: year}

	hour, min, _ := msg.Envelope.Date.Clock()
	lEnv.ReceivedTime = Time{Hour: hour, Minute: min}

	lEnv.Subject = msg.Envelope.Subject
	lEnv.MessageID = msg.Envelope.MessageId

	for _, address := range msg.Envelope.Bcc{
		lEnv.BCC = append(lEnv.BCC, address.Address())
	}

	for _, address := range msg.Envelope.Cc {
		lEnv.CC = append(lEnv.CC, address.Address())
	}
	return &LocalMessage{LocalEnvelope: lEnv}
}

func imapMessagesToLocalMessages(msgs []*imap.Message) []*LocalMessage {
	localMsgs := make([]*LocalMessage, len(msgs))

	for i, msg := range msgs {
		localMsgs[i] = imapMessageToLocalMessage(msg)
	}
	return localMsgs
}


type NotificationSettings struct {
	MaxMessages uint32
	MaxTime int64
}

type Filter interface {
	filter(messages []* LocalMessage) []bool
}

type Selector func(message *LocalMessage, values[] string) bool

type StringFieldFilter struct {
	values []string
	selector Selector
}

func (ff StringFieldFilter) filter(messages [] *LocalMessage) []bool {
	keepArr := make([]bool, len(messages))
	for i, msg := range messages {
		keepArr[i] = ff.selector(msg, ff.values)
	}
	return keepArr
}

type TimeFilter struct {
	intervals []TimeInterval
}

func (tf *TimeFilter) filter(messages [] *LocalMessage) []bool {
	keepArr := make([]bool, len(messages))
	for i, msg := range messages {
		for _, tInt := range tf.intervals {
			keepArr[i] = tInt.inTimeInterval(&msg.LocalEnvelope.ReceivedTime)
		}
	}
	return keepArr
}

func defaultNotificationSettings() *NotificationSettings {
	return &NotificationSettings{MaxMessages: 1, MaxTime: 1800}
}


func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func fetchRecentMessages(client *client.Client, mbox *imap.MailboxStatus, n uint32) []*imap.Message {
	if mbox == nil {
		return nil
	}

	messages := mbox.Messages

	log.Printf("Number of messages: %v", messages)
	n = min(n, messages)
	log.Printf("Number of messages to retrieve: %v", n)

	return fetchMessages(client, messages - n + 1, messages)
}


func fetchMessages(client *client.Client, start uint32, end uint32) []*imap.Message {
	log.Printf("Start: %v", start)
	log.Printf("End: %v", end)

	seqset := &imap.SeqSet{}
	seqset.AddRange(start, end)

	messages := make(chan *imap.Message, end - start + 1)
	done := make(chan error, 1)

	//var section imap.BodySectionName

	go func() {
		done <- client.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	if err := <-done; err != nil {
		log.Fatalf("Mailbox fetching failed: %v", err)
	}

	var localMessages []*imap.Message
	for msg := range messages {
		localMessages = append(localMessages, msg)
	}

	return localMessages
}

func (mc *MailboxCache) cacheMessages(messages []*imap.Message) {
	mc.mux.Lock()

	for _, msg := range messages {
		mc.localMessages = append(mc.localMessages, *imapMessageToLocalMessage(msg))
	}
	mc.mux.Unlock()
}

func boolToInt(x bool) int {
	if x == true {
		return 1
	}
	return 0
}


// this should really be error checked
func sumSlices(a, b []int) []int {
	res := make([]int, len(a))

	for i, _ := range a {
		res[i] = a[i] + b[i]
	}
	return res
}

func boolSliceToIntSlice(b []bool) []int{
	res := make([]int, len(b))
	for i, x := range b {
		res[i] = boolToInt(x)
	}
	return res
}


func filterMessages(filters []Filter, messages []*LocalMessage) []*LocalMessage{
	var voteVec []int

	for i, f := range filters {
		if tmp := f.filter(messages); i == 0 {
			voteVec = boolSliceToIntSlice(tmp)
		} else {
			voteVec = sumSlices(voteVec, boolSliceToIntSlice(tmp))
		}
	}

	var res []*LocalMessage
	for i, msg := range messages {
		if len(voteVec) != 0 && float64(voteVec[i]) / float64(len(voteVec)) >= 0.50 {
			res = append(res, msg)
		}
	}
	return res
}

func lazyClientDaemon(filters []Filter, client *client.Client, nc *NotificationSettings) {
	const restTime = time.Second * 10
	mbox, err := client.Select("INBOX", true)

	if err != nil {
		log.Fatalf("Mailbox could not be selected: %v", err)
	}

	prevMessages := mbox.Messages
	lastTime := time.Now().Unix()

	for {
		client.Noop()

		currentMessages := mbox.Messages
		newMessages := currentMessages - prevMessages

		if newMessages > 0 && (newMessages >=  nc.MaxMessages || time.Now().Unix() - lastTime >= nc.MaxTime) {

			messages := imapMessagesToLocalMessages(fetchMessages(client, prevMessages + 1, currentMessages))
			messages = filterMessages(filters, messages)

			if len(messages) > 0 {

				var msg string
				if len(messages) == 1 {
					msg = fmt.Sprintf("%v new email.", len(messages))
				} else {
					msg = fmt.Sprintf("%v new emails.", len(messages))
				}
				err := beeep.Notify(APP_NAME, msg, APP_ICON_PATH)

				if err != nil {
					log.Fatalf("Push notification failed: %v", err)
				}
			}

			prevMessages = currentMessages
			lastTime = time.Now().Unix()
		}
		time.Sleep(restTime)
	}
}

func clientDaemon(client *client.Client, nc *NotificationSettings,
					cacheMessages func ([]*imap.Message), run chan int, quit chan int) {
	// check number of new messages
	const restTime = time.Second * 1

	<- run
	mbox, err := client.Select("INBOX", true)

	if err != nil {
		log.Fatalf("Mailbox could not be selected: %v", err)
	}

	prevMessages := mbox.Messages
	lastTime := time.Now().Unix()

	for {
		select {
		case <- quit:
			log.Println("Exiting mailbox daemon")
			return
		default:
			client.Noop()

			currentMessages := mbox.Messages
			newMessages := currentMessages - prevMessages

			if newMessages > 0 && (newMessages >=  nc.MaxMessages ||
				time.Now().Unix() - lastTime >= nc.MaxTime) {

				cacheMessages(fetchMessages(client, prevMessages + 1, currentMessages))

				msg := fmt.Sprintf("%v new emails.", newMessages)
				err := beeep.Notify(APP_NAME, msg, APP_ICON_PATH)

				if err != nil {
					log.Fatalf("Push notification failed: %v", err)
				}

				prevMessages = currentMessages
				lastTime = time.Now().Unix()
			}
			time.Sleep(restTime)
		}
	}
}


func printIMAPMessageEnvelope(msg *imap.Message) {
	fmt.Println("----")
	fmt.Printf("Author: %v\n", msg.Envelope.Sender[0].PersonalName)
	fmt.Printf("Email Address: %v\n", msg.Envelope.Sender[0].Address())

	year, month, day := msg.Envelope.Date.Date()
	fmt.Printf("Date: (%v, %v, %v)\n", day, month, year)

	hour, min, sec :=  msg.Envelope.Date.Clock()
	fmt.Printf("Time: (%v, %v, %v)\n", hour, min, sec)
	fmt.Printf("Subject: %v\n", msg.Envelope.Subject)
	fmt.Printf("Message ID: %v\n", msg.Envelope.MessageId)
	fmt.Printf("CC: %v\n", msg.Envelope.Cc)
	fmt.Printf("BCC: %v\n", msg.Envelope.Bcc)
	fmt.Printf("----\n\n")
}


func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func main() {
    	jsonFile, err := os.Open("config.json")
    
    	if err != nil {
        	fmt.Println(err)
    	}
    	
    	defer jsonFile.Close()
	bytes, _ := ioutil.ReadAll(jsonFile)
	
	var config Config
	json.Unmarshal(bytes, &config)


	imapClient, _ := client.DialTLS("imap.gmail.com:993", nil)
	err = imapClient.Login(config.Email, config.AppID)

	if err != nil {
		fmt.Println("Error: invalid config file")
		panic(1)
	}

	fmt.Println("Logged in.")


	var emailFilter Filter = StringFieldFilter{values:config.ValidEmails,
		selector: func(message *LocalMessage, values []string) bool {
			for _, address := range message.LocalEnvelope.SenderAddresses {
				if stringInSlice(address, values) {
					return true
				}
			}
			return false
		},
	}
	var filters []Filter

	filters = append(filters, emailFilter)
	fmt.Println("Launching email client")
	lazyClientDaemon(filters, imapClient, defaultNotificationSettings())
}
