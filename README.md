![alt text](logo.png)


# About
Hush is a simple email filtering app for gmail that minimizes the number of times you need to check your inbox. 

# Requirements 
* Golang
* Either Linux or MacOS (untested)

# Installation 
1. Clone this repository. 
2. Install the following packages using go up: 
  * github.com/emersion/go-imap 
  * github.com/emersion/go-imap/client 
  * github.com/gen2brain/beeep

# How to Use 
1. Enable [imap support](https://support.google.com/mail/answer/7126229?hl=en) for gmail. 
2. Create an [app password](https://support.google.com/accounts/answer/185833?hl=en).
3. Edit **config.json** so that it includes your email address, the app password associated with your email, and the email addresses 
   you want to receive notifications from. 
4. Run:
  go run main.go

