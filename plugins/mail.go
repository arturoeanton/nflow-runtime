package plugins

import (
	"log"
	"net/mail"
	"net/smtp"

	"github.com/labstack/echo/v4"
	"github.com/scorredoira/email"
)

type MailPlugin string
type ConfigMail struct {
	MailSMTP     string `toml:"smtp"`
	MailSMTPPort string `toml:"port"`
	MailFrom     string `toml:"from"`
	MailPassword string `toml:"password"`
}

var (
	fxsMail map[string]interface{} = make(map[string]interface{})
	config  ConfigMail
)

func (d MailPlugin) Run(c echo.Context,
	vars map[string]string, payloadIn interface{}, dromaderyData string,
	callback chan string,
) (payloadOut interface{}, next string, err error) {
	return nil, "output_1", nil
}

func (d MailPlugin) AddFeatureJS() map[string]interface{} {
	return fxsMail
}

func (d MailPlugin) Name() string {
	return "mail"
}

func init() {
	fxsMail["send_mail"] = SendMail
	fxsMail["send_mail_async"] = SendMailAsync
	fxsMail["mail_config"] = func(smtp string, port string, from string, password string) {
		config = ConfigMail{
			MailSMTP:     smtp,
			MailSMTPPort: port,
			MailFrom:     from,
			MailPassword: password,
		}
	}
}

func sendMail(mailTo string, subject string, msg string, attach string, format string) {

	if config.MailPassword == "" || config.MailSMTP == "" || config.MailFrom == "" {
		log.Println("mail disabled")
		return
	}
	if config.MailSMTPPort == "" {
		config.MailSMTPPort = "25"
	}
	auth := smtp.PlainAuth("", config.MailFrom, config.MailPassword, config.MailSMTP)

	m := email.NewMessage(subject, msg)
	if format == "html" {
		m = email.NewHTMLMessage(subject, msg)
	}

	m.From = mail.Address{Name: "From", Address: config.MailFrom}
	m.To = []string{mailTo}
	if attach != "" {
		if err := m.Attach(attach); err != nil {
			log.Println(err)
			return
		}
	}
	server := config.MailSMTP + ":" + config.MailSMTPPort
	if err := email.Send(server, auth, m); err != nil {
		log.Println(err)
		return
	}
}

// SendMail return function for send mail
func SendMail(mailTo string, subject string, msg string, format string, attach string) {
	sendMail(mailTo, subject, msg, format, attach)
}

// SendMailAsync return function for send mail async
func SendMailAsync(mailTo string, subject string, msg string, format string, attach string) {
	go sendMail(mailTo, subject, msg, format, attach)
}
