package mailer

import (
	"bytes"
	"embed"
	mail "github.com/xhit/go-simple-mail/v2"
	"html/template"
	"myapp/internal/config"
	"path"
	"time"
)

var emailTemplatesFS embed.FS

// SetEmailTemplatesFS email-templatesを使い分けるための関数(初期化処理でgo:embedを定義する必要あり)
func SetEmailTemplatesFS(fs embed.FS) {
	emailTemplatesFS = fs
}

func SendMail(from, to, subject, tmpl string, attachments []string, data interface{}) error {
	templateToRender := path.Join("email-templates", tmpl+".html.tmpl")

	t, err := template.New("email-html").ParseFS(emailTemplatesFS, templateToRender)
	if err != nil {
		return err
	}

	var tpl bytes.Buffer
	if err = t.ExecuteTemplate(&tpl, "body", data); err != nil {
		return err
	}

	formattedMessage := tpl.String()
	templateToRender = path.Join("email-templates", tmpl+".plain.tmpl")
	t, err = template.New("email-plain").ParseFS(emailTemplatesFS, templateToRender)
	if err != nil {
		return err
	}

	if err = t.ExecuteTemplate(&tpl, "body", data); err != nil {
		return err
	}

	plainMessage := tpl.String()
	loadConfig := config.LoadConfig()

	server := mail.NewSMTPClient()
	server.Host = loadConfig.SMTP.Host
	server.Port = loadConfig.SMTP.Port
	server.Username = loadConfig.SMTP.Username
	server.Password = loadConfig.SMTP.Password
	server.Encryption = mail.EncryptionTLS
	server.KeepAlive = false
	server.ConnectTimeout = 10 * time.Second
	server.SendTimeout = 10 * time.Second

	smtpClient, err := server.Connect()
	if err != nil {
		return err
	}

	email := mail.NewMSG()
	email.SetFrom(from).AddTo(to).SetSubject(subject)
	email.SetBody(mail.TextHTML, formattedMessage)
	email.AddAlternative(mail.TextPlain, plainMessage)

	if len(attachments) > 0 {
		for _, attachment := range attachments {
			email.AddAttachment(attachment)
		}
	}

	err = email.Send(smtpClient)
	if err != nil {
		return err
	}

	return nil
}
