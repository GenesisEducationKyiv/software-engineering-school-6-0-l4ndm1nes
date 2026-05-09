package domain

import "errors"

var (
	ErrInvalidRepoFormat = errors.New("invalid repository format, expected owner/repo")
	ErrInvalidEmail      = errors.New("invalid email address")
	ErrRepoNotFound      = errors.New("repository not found on GitHub")
	ErrAlreadySubscribed = errors.New("email already subscribed to this repository")
	ErrTokenNotFound     = errors.New("token not found")
	ErrInvalidToken      = errors.New("invalid token")
	ErrRateLimited       = errors.New("GitHub API rate limit exceeded")
)
