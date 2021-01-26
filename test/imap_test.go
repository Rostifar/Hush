package main

import (
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"testing"
	"time"
)


func TestFetchRecentMessages(t *testing.T) {
	imapClient, _ := client.DialTLS("imap.gmail.com:993", nil)
	err := imapClient.Login("rcbridendev@gmail.com", "lkzsemypzcxpvjhg")

	if err != nil {
		t.Errorf("Client creation failed: %v", err)
	}

	mbox, err := imapClient.Select("INBOX", true)

	if err != nil {
		t.Errorf("Mailbox selection failed: %v", err)
	}

	messages := fetchRecentMessages(imapClient, mbox, 5)

	for _, msg := range messages {
		printIMAPMessageEnvelope(msg)
	}
}


func TestClientDaemon(t *testing.T) {
	imapClient, _ := client.DialTLS("imap.gmail.com:993", nil)
	err := imapClient.Login("rcbridendev@gmail.com", "lkzsemypzcxpvjhg")

	if err != nil {
		t.Errorf("Client creation failed: %v", err)
	}

	run := make(chan int, 1)
	quit := make(chan int, 1)

	run <- 1
	go clientDaemon(imapClient, &NotificationSettings{MaxMessages: 2, MaxTime: 30},
		func (m []*imap.Message) {
			t.Logf("Messages received: %v", len(m))
		}, run, quit)

	time.Sleep(time.Second * 60)
	quit <- 1
}

