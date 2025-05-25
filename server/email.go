package main

import (
	"errors"
	"log/slog"

	sendgrid "github.com/sendgrid/sendgrid-go"
	mail "github.com/sendgrid/sendgrid-go/helpers/mail"
)

var (
	SENDER_EMAIL            = "noreply@tunnels.is"
	SENDER_NAME             = "Tunnels"
	SENDER_TITLE            = "Tunnels"
	PASSWORD_RESET_TEMPLATE = "d-1fae9ba709454763bd52eae766e2d202"
	CONFIRM_TEMPLATE        = "d-70654c5772f646429d4800a0394079ec"
	INVOICE_TEMPLATE        = "d-38dd72e47f904ae3bbe46316a65c4625"
	API_URL                 = "https://api.sendgrid.com"
)

func SEND_PASSWORD_RESET(key string, email string, code string) error {
	m := mail.NewV3Mail()
	e := mail.NewEmail(SENDER_NAME, SENDER_EMAIL)
	m.SetFrom(e)
	m.SetTemplateID(PASSWORD_RESET_TEMPLATE)
	m.AddAttachment()

	p := mail.NewPersonalization()
	tos := []*mail.Email{
		mail.NewEmail(SENDER_TITLE, email),
	}
	p.AddTos(tos...)
	p.SetDynamicTemplateData("resetCode", code)
	m.AddPersonalizations(p)

	request := sendgrid.GetRequest(key, "/v3/mail/send", API_URL)
	request.Method = "POST"
	Body := mail.GetRequestBody(m)
	request.Body = Body

	response, err := sendgrid.API(request)
	if err != nil {
		logger.Error("sendgrid error", slog.Any("resp", response))
		return err
	} else if response.StatusCode != 202 {
		logger.Error("sendgrid non-202 code", slog.Any("resp", response))
		return errors.New("SENDGRID NON 202 STATUS CODE")
	}

	return nil
}

func SEND_CONFIRMATION(key string, email string, code string) error {
	m := mail.NewV3Mail()
	e := mail.NewEmail(SENDER_NAME, SENDER_EMAIL)
	m.SetFrom(e)
	m.SetTemplateID(CONFIRM_TEMPLATE)

	p := mail.NewPersonalization()
	tos := []*mail.Email{
		mail.NewEmail(SENDER_TITLE, email),
	}
	p.AddTos(tos...)
	p.SetDynamicTemplateData("confirmCode", code)
	m.AddPersonalizations(p)

	request := sendgrid.GetRequest(key, "/v3/mail/send", API_URL)
	request.Method = "POST"
	Body := mail.GetRequestBody(m)
	request.Body = Body

	response, err := sendgrid.API(request)
	if err != nil {
		logger.Error("sendgrid error", slog.Any("resp", response))
		return err
	} else if response.StatusCode != 202 {
		logger.Error("sendgrid non-202 code", slog.Any("resp", response))
		return errors.New("SENDGRID NON 202 STATUS CODE")
	}

	return nil
}
