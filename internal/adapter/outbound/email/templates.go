package email

import "fmt"

type Message struct {
	Subject  string
	HTMLBody string
}

func RenderConfirmation(repo, confirmURL string) Message {
	return Message{
		Subject: fmt.Sprintf("Confirm your subscription to %s releases", repo),
		HTMLBody: fmt.Sprintf(
			`<html><body>
<h2>Confirm Your Subscription</h2>
<p>You have requested to receive release notifications for <strong>%s</strong>.</p>
<p>Please confirm your subscription by clicking the link below:</p>
<p><a href="%s">Confirm Subscription</a></p>
<p>If you did not request this, you can safely ignore this email.</p>
</body></html>`, repo, confirmURL),
	}
}

func RenderReleaseNotification(repo, tag, releaseURL, unsubscribeURL string) Message {
	return Message{
		Subject: fmt.Sprintf("New release %s for %s", tag, repo),
		HTMLBody: fmt.Sprintf(
			`<html><body>
<h2>New Release: %s</h2>
<p>Repository <strong>%s</strong> has a new release: <strong>%s</strong></p>
<p><a href="%s">View Release on GitHub</a></p>
<hr>
<p><small><a href="%s">Unsubscribe</a> from release notifications for this repository.</small></p>
</body></html>`, tag, repo, tag, releaseURL, unsubscribeURL),
	}
}
