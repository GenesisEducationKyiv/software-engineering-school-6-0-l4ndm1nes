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
	return s.send(ctx, email, RenderConfirmation(repo, confirmURL))
}

func (s *Sender) SendReleaseNotification(ctx context.Context, email, repo, tag, releaseURL, unsubscribeURL string) error {
	return s.send(ctx, email, RenderReleaseNotification(repo, tag, releaseURL, unsubscribeURL))
}

func (s *Sender) send(ctx context.Context, to string, msg Message) error {
	rawMsg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\n"+
			"MIME-Version: 1.0\r\nContent-Type: text/html; charset=\"UTF-8\"\r\n\r\n%s",
		s.from, to, msg.Subject, msg.HTMLBody,
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

	s.logger.Info("email sent", "to", to, "subject", msg.Subject)
	return nil
}
