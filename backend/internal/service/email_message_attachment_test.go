//go:build unit

package service

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func attachmentTestConfig() *SMTPConfig {
	return &SMTPConfig{Host: "smtp.example.com", From: "noreply@example.com", FromName: "CodePlan"}
}

func TestBuildSMTPMessageWithAttachmentIsMultipartMixed(t *testing.T) {
	pdf := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte{0, 1, 2, 250, 255}, 100)...)
	message, err := buildSMTPMessageWithAttachment(attachmentTestConfig(), "user@example.net", "发票", "<p>您好</p>",
		&EmailAttachment{Filename: "发票 26332.pdf", ContentType: "application/pdf", Data: pdf})
	require.NoError(t, err)

	parsed, err := mail.ReadMessage(bytes.NewReader(message.data))
	require.NoError(t, err)
	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", mediaType)

	reader := multipart.NewReader(parsed.Body, params["boundary"])

	bodyPart, err := reader.NextRawPart()
	require.NoError(t, err)
	require.Equal(t, "quoted-printable", bodyPart.Header.Get("Content-Transfer-Encoding"))
	text, err := io.ReadAll(quotedprintable.NewReader(bodyPart))
	require.NoError(t, err)
	require.Equal(t, "<p>您好</p>", string(text))

	filePart, err := reader.NextRawPart()
	require.NoError(t, err)
	require.Equal(t, "base64", filePart.Header.Get("Content-Transfer-Encoding"))
	partType, typeParams, err := mime.ParseMediaType(filePart.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "application/pdf", partType)
	require.Equal(t, "发票 26332.pdf", typeParams["name"])
	disposition, dispositionParams, err := mime.ParseMediaType(filePart.Header.Get("Content-Disposition"))
	require.NoError(t, err)
	require.Equal(t, "attachment", disposition)
	require.Equal(t, "发票 26332.pdf", dispositionParams["filename"])

	encoded, err := io.ReadAll(filePart)
	require.NoError(t, err)
	for _, line := range strings.Split(string(encoded), "\r\n") {
		require.LessOrEqual(t, len(line), 76, "base64 lines must stay within the SMTP-safe width")
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(encoded), "\r\n", ""))
	require.NoError(t, err)
	require.Equal(t, pdf, decoded)

	_, err = reader.NextRawPart()
	require.Equal(t, io.EOF, err)
}

func TestBuildSMTPMessageWithoutAttachmentStaysSinglePartHTML(t *testing.T) {
	message, err := buildSMTPMessageWithAttachment(attachmentTestConfig(), "user@example.net", "subject", "<p>hi</p>", nil)
	require.NoError(t, err)

	parsed, err := mail.ReadMessage(bytes.NewReader(message.data))
	require.NoError(t, err)
	require.Equal(t, "text/html; charset=UTF-8", parsed.Header.Get("Content-Type"))
	require.NotContains(t, string(message.data), "multipart")
}

func TestBuildSMTPMessageAttachmentNameCannotInjectHeaders(t *testing.T) {
	message, err := buildSMTPMessageWithAttachment(attachmentTestConfig(), "user@example.net", "subject", "body",
		&EmailAttachment{Filename: "a\r\nBcc: hidden@example.com\r\n.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.7")})
	require.NoError(t, err)

	require.NotContains(t, string(message.data), "\r\nBcc:")
	parsed, err := mail.ReadMessage(bytes.NewReader(message.data))
	require.NoError(t, err)
	require.Empty(t, parsed.Header.Get("Bcc"))
}
