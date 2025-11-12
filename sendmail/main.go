package main

import (
	"log"
	"net/smtp"
)

func main() {
	from := "test@example.com"
	to := []string{"hello@yopmail.com"}
	msg := []byte("To: hello@yopmail.com\r\n" +
		"Subject: Mailpit Test\r\n" +
		"\r\n" +
		"This is a test email sent via Mailpit SMTP.\r\n")

	err := smtp.SendMail("localhost:2025", nil, from, to, msg)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Mail sent successfully!")
}
