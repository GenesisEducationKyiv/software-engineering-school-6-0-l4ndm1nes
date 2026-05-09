package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"time"
)

type Sender struct {
	host     string
	port     string
	user     string
	password string
	from     string
	timeout  time.Duration
	logger   *slog.Logger
}

func NewSender(host, port, user, password, from string, timeout time.Duration, logger *slog.Logger) *Sender {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Sender{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
		timeout:  timeout,
		logger:   logger,
	}
}

func (s *Sender) SendConfirmation(ctx context.Context, email, repo, confirmURL string) error {
	subject := fmt.Sprintf("Confirm your subscription to %s releases", repo)
	body := fmt.Sprintf(
		`<html><body>
<h2>Confirm Your Subscription</h2>
<p>You have requested to receive release notifications for <strong>%s</strong>.</p>
<p>Please confirm your subscription by clicking the link below:</p>
<p><a href="%s">Confirm Subscription</a></p>
<p>If you did not request this, you can safely ignore this email.</p>
</body></html>`, repo, confirmURL)

	return s.sendEmail(ctx, email, subject, body)
}

func (s *Sender) SendReleaseNotification(ctx context.Context, email, repo, tag, releaseURL, unsubscribeURL string) error {
	subject := fmt.Sprintf("New release %s for %s", tag, repo)
	body := fmt.Sprintf(
		`<html><body>
<h2>New Release: %s</h2>
<p>Repository <strong>%s</strong> has a new release: <strong>%s</strong></p>
<p><a href="%s">View Release on GitHub</a></p>
<hr>
<p><small><a href="%s">Unsubscribe</a> from release notifications for this repository.</small></p>
</body></html>`, tag, repo, tag, releaseURL, unsubscribeURL)

	return s.sendEmail(ctx, email, subject, body)
}

func (s *Sender) sendEmail(ctx context.Context, to, subject, htmlBody string) error {
	rawMsg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n"+
			"MIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		s.from, to, subject, htmlBody,
	)

	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(s.timeout)
	}

	dialer := &net.Dialer{
		Timeout:  s.timeout,
		Deadline: deadline,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
			s.logger.Debug("STARTTLS not available, continuing plaintext", "error", err)
		}
	}

	if s.user != "" && s.password != "" {
		if err := client.Auth(smtp.PlainAuth("", s.user, s.password, s.host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write([]byte(rawMsg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}

	if err := client.Quit(); err != nil {
		s.logger.Debug("smtp QUIT failed", "error", err)
	}

	s.logger.Info("email sent", "to", to, "subject", subject)
	return nil
}
