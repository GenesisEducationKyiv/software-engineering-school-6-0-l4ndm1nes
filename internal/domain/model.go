package domain

import "time"

type Repository struct {
	ID          int
	Owner       string
	Repo        string
	LastSeenTag string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *Repository) FullName() string {
	return r.Owner + "/" + r.Repo
}

type Subscription struct {
	ID               int
	Email            string
	RepositoryID     int
	Confirmed        bool
	ConfirmToken     string
	UnsubscribeToken string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Repository       *Repository
}

type Release struct {
	TagName     string
	Name        string
	HTMLURL     string
	PublishedAt time.Time
}
