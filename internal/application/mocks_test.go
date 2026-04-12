package application

import (
	"context"
	"time"

	"github.com/user/github-release-notification-api/internal/domain"
)

// --- Mock SubscriptionRepository ---

type mockRepo struct {
	repos         map[string]*domain.Repository
	subs          map[int]*domain.Subscription
	subsByEmail   map[string][]int
	subsByConfirm map[string]int
	subsByUnsub   map[string]int
	subsByRepo    map[int][]int
	nextRepoID    int
	nextSubID     int

	createRepoErr error
	createSubErr  error
	confirmErr    error
	deleteErr     error
	getByEmailErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		repos:         make(map[string]*domain.Repository),
		subs:          make(map[int]*domain.Subscription),
		subsByEmail:   make(map[string][]int),
		subsByConfirm: make(map[string]int),
		subsByUnsub:   make(map[string]int),
		subsByRepo:    make(map[int][]int),
		nextRepoID:    1,
		nextSubID:     1,
	}
}

func (m *mockRepo) CreateRepository(_ context.Context, owner, repo string) (*domain.Repository, error) {
	if m.createRepoErr != nil {
		return nil, m.createRepoErr
	}
	key := owner + "/" + repo
	if r, ok := m.repos[key]; ok {
		return r, nil
	}
	r := &domain.Repository{
		ID:        m.nextRepoID,
		Owner:     owner,
		Repo:      repo,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.nextRepoID++
	m.repos[key] = r
	return r, nil
}

func (m *mockRepo) GetRepositoryByOwnerRepo(_ context.Context, owner, repo string) (*domain.Repository, error) {
	key := owner + "/" + repo
	return m.repos[key], nil
}

func (m *mockRepo) UpdateLastSeenTag(_ context.Context, repoID int, tag string) error {
	for _, r := range m.repos {
		if r.ID == repoID {
			r.LastSeenTag = tag
			return nil
		}
	}
	return nil
}

func (m *mockRepo) GetRepositoriesWithConfirmedSubs(_ context.Context) ([]domain.Repository, error) {
	var result []domain.Repository
	for _, r := range m.repos {
		if ids, ok := m.subsByRepo[r.ID]; ok && len(ids) > 0 {
			for _, id := range ids {
				if s, ok := m.subs[id]; ok && s.Confirmed {
					result = append(result, *r)
					break
				}
			}
		}
	}
	return result, nil
}

func (m *mockRepo) CreateSubscription(_ context.Context, sub *domain.Subscription) error {
	if m.createSubErr != nil {
		return m.createSubErr
	}
	sub.ID = m.nextSubID
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	m.nextSubID++
	m.subs[sub.ID] = sub
	m.subsByEmail[sub.Email] = append(m.subsByEmail[sub.Email], sub.ID)
	m.subsByConfirm[sub.ConfirmToken] = sub.ID
	m.subsByUnsub[sub.UnsubscribeToken] = sub.ID
	m.subsByRepo[sub.RepositoryID] = append(m.subsByRepo[sub.RepositoryID], sub.ID)
	return nil
}

func (m *mockRepo) GetSubscriptionByEmailAndRepo(_ context.Context, email string, repoID int) (*domain.Subscription, error) {
	for _, id := range m.subsByEmail[email] {
		if s, ok := m.subs[id]; ok && s.RepositoryID == repoID {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) GetSubscriptionsByEmail(_ context.Context, email string) ([]domain.Subscription, error) {
	if m.getByEmailErr != nil {
		return nil, m.getByEmailErr
	}
	var result []domain.Subscription
	for _, id := range m.subsByEmail[email] {
		if s, ok := m.subs[id]; ok {
			sub := *s
			for _, r := range m.repos {
				if r.ID == sub.RepositoryID {
					sub.Repository = r
					break
				}
			}
			result = append(result, sub)
		}
	}
	return result, nil
}

func (m *mockRepo) GetSubscriptionByConfirmToken(_ context.Context, token string) (*domain.Subscription, error) {
	if id, ok := m.subsByConfirm[token]; ok {
		return m.subs[id], nil
	}
	return nil, nil
}

func (m *mockRepo) GetSubscriptionByUnsubscribeToken(_ context.Context, token string) (*domain.Subscription, error) {
	if id, ok := m.subsByUnsub[token]; ok {
		return m.subs[id], nil
	}
	return nil, nil
}

func (m *mockRepo) ConfirmSubscription(_ context.Context, id int) error {
	if m.confirmErr != nil {
		return m.confirmErr
	}
	if s, ok := m.subs[id]; ok {
		s.Confirmed = true
	}
	return nil
}

func (m *mockRepo) DeleteSubscription(_ context.Context, id int) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.subs, id)
	return nil
}

func (m *mockRepo) GetConfirmedSubscriptionsByRepoID(_ context.Context, repoID int) ([]domain.Subscription, error) {
	var result []domain.Subscription
	for _, id := range m.subsByRepo[repoID] {
		if s, ok := m.subs[id]; ok && s.Confirmed {
			result = append(result, *s)
		}
	}
	return result, nil
}

// --- Mock GitHubClient ---

type mockGitHub struct {
	existingRepos map[string]bool
	releases      map[string]*domain.Release
	repoExistsErr error
	releaseErr    error
}

func newMockGitHub() *mockGitHub {
	return &mockGitHub{
		existingRepos: make(map[string]bool),
		releases:      make(map[string]*domain.Release),
	}
}

func (m *mockGitHub) RepoExists(_ context.Context, owner, repo string) (bool, error) {
	if m.repoExistsErr != nil {
		return false, m.repoExistsErr
	}
	return m.existingRepos[owner+"/"+repo], nil
}

func (m *mockGitHub) GetLatestRelease(_ context.Context, owner, repo string) (*domain.Release, error) {
	if m.releaseErr != nil {
		return nil, m.releaseErr
	}
	return m.releases[owner+"/"+repo], nil
}

// --- Mock Mailer ---

type mockMailer struct {
	confirmationsSent   []sentConfirmation
	notificationsSent   []sentNotification
	sendConfirmErr      error
	sendNotificationErr error
}

type sentConfirmation struct {
	Email      string
	Repo       string
	ConfirmURL string
}

type sentNotification struct {
	Email          string
	Repo           string
	Tag            string
	ReleaseURL     string
	UnsubscribeURL string
}

func newMockMailer() *mockMailer {
	return &mockMailer{}
}

func (m *mockMailer) SendConfirmation(_ context.Context, email, repo, confirmURL string) error {
	if m.sendConfirmErr != nil {
		return m.sendConfirmErr
	}
	m.confirmationsSent = append(m.confirmationsSent, sentConfirmation{
		Email:      email,
		Repo:       repo,
		ConfirmURL: confirmURL,
	})
	return nil
}

func (m *mockMailer) SendReleaseNotification(_ context.Context, email, repo, tag, releaseURL, unsubscribeURL string) error {
	if m.sendNotificationErr != nil {
		return m.sendNotificationErr
	}
	m.notificationsSent = append(m.notificationsSent, sentNotification{
		Email:          email,
		Repo:           repo,
		Tag:            tag,
		ReleaseURL:     releaseURL,
		UnsubscribeURL: unsubscribeURL,
	})
	return nil
}
