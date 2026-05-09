package port

import "context"

type Mailer interface {
	SendConfirmation(ctx context.Context, email, repo, confirmURL string) error
	SendReleaseNotification(ctx context.Context, email, repo, tag, releaseURL, unsubscribeURL string) error
}
