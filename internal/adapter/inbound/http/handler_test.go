package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/github-release-notification-api/internal/application"
	"github.com/user/github-release-notification-api/internal/domain"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- Mocks ---

type mockRepo struct {
	repos      map[string]*domain.Repository
	subs       map[int]*domain.Subscription
	nextRepoID int
	nextSubID  int
	byEmail    map[string][]int
	byConfirm  map[string]int
	byUnsub    map[string]int
	byRepoID   map[int][]int
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		repos:      make(map[string]*domain.Repository),
		subs:       make(map[int]*domain.Subscription),
		nextRepoID: 1,
		nextSubID:  1,
		byEmail:    make(map[string][]int),
		byConfirm:  make(map[string]int),
		byUnsub:    make(map[string]int),
		byRepoID:   make(map[int][]int),
	}
}

func (m *mockRepo) CreateRepository(_ context.Context, owner, repo string) (*domain.Repository, error) {
	key := owner + "/" + repo
	if r, ok := m.repos[key]; ok {
		return r, nil
	}
	r := &domain.Repository{ID: m.nextRepoID, Owner: owner, Repo: repo, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.nextRepoID++
	m.repos[key] = r
	return r, nil
}
func (m *mockRepo) GetRepositoryByOwnerRepo(_ context.Context, owner, repo string) (*domain.Repository, error) {
	return m.repos[owner+"/"+repo], nil
}
func (m *mockRepo) UpdateLastSeenTag(_ context.Context, _ int, _ string) error { return nil }
func (m *mockRepo) GetRepositoriesWithConfirmedSubs(_ context.Context) ([]domain.Repository, error) {
	return nil, nil
}
func (m *mockRepo) CreateSubscription(_ context.Context, sub *domain.Subscription) error {
	sub.ID = m.nextSubID
	m.nextSubID++
	m.subs[sub.ID] = sub
	m.byEmail[sub.Email] = append(m.byEmail[sub.Email], sub.ID)
	m.byConfirm[sub.ConfirmToken] = sub.ID
	m.byUnsub[sub.UnsubscribeToken] = sub.ID
	m.byRepoID[sub.RepositoryID] = append(m.byRepoID[sub.RepositoryID], sub.ID)
	return nil
}
func (m *mockRepo) GetSubscriptionByEmailAndRepo(_ context.Context, email string, repoID int) (*domain.Subscription, error) {
	for _, id := range m.byEmail[email] {
		if s := m.subs[id]; s != nil && s.RepositoryID == repoID {
			return s, nil
		}
	}
	return nil, nil
}
func (m *mockRepo) GetSubscriptionsByEmail(_ context.Context, email string) ([]domain.Subscription, error) {
	var result []domain.Subscription
	for _, id := range m.byEmail[email] {
		if s := m.subs[id]; s != nil {
			sub := *s
			for _, r := range m.repos {
				if r.ID == sub.RepositoryID {
					sub.Repository = r
				}
			}
			result = append(result, sub)
		}
	}
	return result, nil
}
func (m *mockRepo) GetSubscriptionByConfirmToken(_ context.Context, token string) (*domain.Subscription, error) {
	if id, ok := m.byConfirm[token]; ok {
		return m.subs[id], nil
	}
	return nil, nil
}
func (m *mockRepo) GetSubscriptionByUnsubscribeToken(_ context.Context, token string) (*domain.Subscription, error) {
	if id, ok := m.byUnsub[token]; ok {
		return m.subs[id], nil
	}
	return nil, nil
}
func (m *mockRepo) ConfirmSubscription(_ context.Context, id int) error {
	if s := m.subs[id]; s != nil {
		s.Confirmed = true
	}
	return nil
}
func (m *mockRepo) DeleteSubscription(_ context.Context, id int) error {
	delete(m.subs, id)
	return nil
}
func (m *mockRepo) GetConfirmedSubscriptionsByRepoID(_ context.Context, _ int) ([]domain.Subscription, error) {
	return nil, nil
}

type mockGitHub struct {
	existing map[string]bool
}

func (m *mockGitHub) RepoExists(_ context.Context, owner, repo string) (bool, error) {
	return m.existing[owner+"/"+repo], nil
}
func (m *mockGitHub) GetLatestRelease(_ context.Context, _, _ string) (*domain.Release, error) {
	return nil, nil
}

type mockMailer struct{}

func (m *mockMailer) SendConfirmation(_ context.Context, _, _, _ string) error { return nil }
func (m *mockMailer) SendReleaseNotification(_ context.Context, _, _, _, _, _ string) error {
	return nil
}

func setupTestRouter() (*gin.Engine, *mockRepo) {
	repo := newMockRepo()
	gh := &mockGitHub{existing: map[string]bool{"golang/go": true}}
	mailer := &mockMailer{}

	svc := application.NewSubscriptionService(
		repo, gh, mailer,
		application.NewCryptoTokenGenerator(0),
		application.NewURLBuilder("http://localhost:8080"),
		testLogger(),
	)
	handler := NewHandler(svc)

	router := gin.New()
	api := router.Group("/api")
	api.POST("/subscribe", handler.Subscribe)
	api.GET("/confirm/:token", handler.Confirm)
	api.GET("/unsubscribe/:token", handler.Unsubscribe)
	api.GET("/subscriptions", handler.GetSubscriptions)

	return router, repo
}

func TestHandler_Subscribe_Success(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	body := `{"email":"test@example.com","repo":"golang/go"}`
	req, _ := http.NewRequest("POST", "/api/subscribe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Subscribe_InvalidRepo(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	body := `{"email":"test@example.com","repo":"invalid"}`
	req, _ := http.NewRequest("POST", "/api/subscribe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_Subscribe_RepoNotFound(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	body := `{"email":"test@example.com","repo":"nonexistent/repo"}`
	req, _ := http.NewRequest("POST", "/api/subscribe", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_Subscribe_Conflict(t *testing.T) {
	router, _ := setupTestRouter()

	body := `{"email":"test@example.com","repo":"golang/go"}`

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/subscribe", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w1, req1)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/subscribe", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w2.Code)
	}
}

func TestHandler_Confirm_NotFound(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/confirm/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_Unsubscribe_NotFound(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/unsubscribe/nonexistent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetSubscriptions_Success(t *testing.T) {
	router, _ := setupTestRouter()

	// Subscribe first
	w1 := httptest.NewRecorder()
	body := `{"email":"test@example.com","repo":"golang/go"}`
	req1, _ := http.NewRequest("POST", "/api/subscribe", strings.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w1, req1)

	// Get subscriptions
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/subscriptions?email=test@example.com", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var subs []subscriptionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &subs); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(subs))
	}
}

func TestHandler_GetSubscriptions_InvalidEmail(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/subscriptions?email=invalid", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetSubscriptions_MissingEmail(t *testing.T) {
	router, _ := setupTestRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/subscriptions", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
