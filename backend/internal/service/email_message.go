package service

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
)

type smtpMessage struct {
	envelopeFrom string
	envelopeTo   string
	data         []byte
}

// EmailAttachment 是随邮件发送的单个附件。
type EmailAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

func buildSMTPMessage(config *SMTPConfig, to, subject, body string) (smtpMessage, error) {
	return buildSMTPMessageWithAttachment(config, to, subject, body, nil)
}

// buildSMTPMessageWithAttachment 在 attachment 为 nil 时与原来的单部分 HTML 邮件完全一致；
// 有附件时改为 multipart/mixed：第一部分是 HTML 正文，第二部分是 base64 编码的附件。
func buildSMTPMessageWithAttachment(config *SMTPConfig, to, subject, body string, attachment *EmailAttachment) (smtpMessage, error) {
	if config == nil {
		return smtpMessage{}, errors.New("missing SMTP configuration")
	}

	fromAddress, err := parseSMTPAddress(config.From, "from")
	if err != nil {
		return smtpMessage{}, err
	}
	recipientAddress, err := parseSMTPAddress(to, "recipient")
	if err != nil {
		return smtpMessage{}, err
	}
	messageID, err := generateEmailMessageID(fromAddress.Address, config.Host)
	if err != nil {
		return smtpMessage{}, fmt.Errorf("generate message ID: %w", err)
	}

	fromName := sanitizeEmailHeader(config.FromName)
	if strings.TrimSpace(fromName) == "" {
		fromName = fromAddress.Name
	}
	fromHeader := (&mail.Address{
		Name:    fromName,
		Address: fromAddress.Address,
	}).String()
	toHeader := (&mail.Address{
		Name:    recipientAddress.Name,
		Address: recipientAddress.Address,
	}).String()
	subjectHeader := mime.QEncoding.Encode("UTF-8", sanitizeEmailHeader(subject))

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&message, "To: %s\r\n", toHeader)
	fmt.Fprintf(&message, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&message, "Message-ID: %s\r\n", messageID)
	fmt.Fprintf(&message, "Subject: %s\r\n", subjectHeader)

	boundary := ""
	if attachment == nil {
		fmt.Fprint(&message, "MIME-Version: 1.0\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n"+
			"Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	} else {
		boundary, err = generateEmailBoundary()
		if err != nil {
			return smtpMessage{}, fmt.Errorf("generate MIME boundary: %w", err)
		}
		fmt.Fprintf(&message, "MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n", boundary)
		fmt.Fprintf(&message, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n", boundary)
	}

	bodyWriter := quotedprintable.NewWriter(&message)
	if _, err := bodyWriter.Write([]byte(body)); err != nil {
		return smtpMessage{}, fmt.Errorf("encode email body: %w", err)
	}
	if err := bodyWriter.Close(); err != nil {
		return smtpMessage{}, fmt.Errorf("close email body encoder: %w", err)
	}

	if attachment != nil {
		if err := writeEmailAttachment(&message, boundary, attachment); err != nil {
			return smtpMessage{}, err
		}
	}

	return smtpMessage{
		envelopeFrom: fromAddress.Address,
		envelopeTo:   recipientAddress.Address,
		data:         message.Bytes(),
	}, nil
}

// writeEmailAttachment 写入附件部分和结束边界。文件名走 RFC 2231 编码，中文名不会乱码。
func writeEmailAttachment(message *bytes.Buffer, boundary string, attachment *EmailAttachment) error {
	filename := sanitizeEmailHeader(attachment.Filename)
	if strings.TrimSpace(filename) == "" {
		filename = "attachment"
	}
	contentType := strings.TrimSpace(attachment.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	typeHeader := mime.FormatMediaType(contentType, map[string]string{"name": filename})
	dispositionHeader := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	if typeHeader == "" || dispositionHeader == "" {
		return errors.New("invalid email attachment name or type")
	}

	fmt.Fprintf(message, "\r\n--%s\r\n", boundary)
	fmt.Fprintf(message, "Content-Type: %s\r\n", typeHeader)
	fmt.Fprint(message, "Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(message, "Content-Disposition: %s\r\n\r\n", dispositionHeader)

	encoded := base64.StdEncoding.EncodeToString(attachment.Data)
	for len(encoded) > 76 {
		message.WriteString(encoded[:76])
		message.WriteString("\r\n")
		encoded = encoded[76:]
	}
	message.WriteString(encoded)
	fmt.Fprintf(message, "\r\n--%s--\r\n", boundary)
	return nil
}

func generateEmailBoundary() (string, error) {
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return "", err
	}
	return "sub2api-" + hex.EncodeToString(randomID), nil
}

func parseSMTPAddress(value, field string) (*mail.Address, error) {
	if strings.ContainsAny(value, "\r\n") {
		return nil, fmt.Errorf("invalid SMTP %s address: contains a line break", field)
	}

	cleaned := strings.TrimSpace(value)
	address, err := mail.ParseAddress(cleaned)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		if err == nil {
			err = fmt.Errorf("address is empty")
		}
		return nil, fmt.Errorf("invalid SMTP %s address: %w", field, err)
	}
	return address, nil
}

func generateEmailMessageID(fromAddress, smtpHost string) (string, error) {
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return "", err
	}

	domain := strings.TrimSpace(sanitizeEmailHeader(smtpHost))
	if at := strings.LastIndexByte(fromAddress, '@'); at >= 0 && at < len(fromAddress)-1 {
		domain = fromAddress[at+1:]
	}
	domain = strings.Trim(domain, "[]<>")
	if domain == "" {
		domain = "localhost"
	}

	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(randomID), domain), nil
}
