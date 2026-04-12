package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/user/github-release-notification-api/internal/domain"
)

type SubscriptionRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
}

func NewSubscriptionRepo(db *sql.DB, queryTimeout time.Duration) *SubscriptionRepo {
	if queryTimeout <= 0 {
		queryTimeout = 5 * time.Second
	}
	return &SubscriptionRepo{db: db, queryTimeout: queryTimeout}
}

func (r *SubscriptionRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, r.queryTimeout)
}

func (r *SubscriptionRepo) CreateRepository(ctx context.Context, owner, repo string) (*domain.Repository, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var rep domain.Repository
	var tag sql.NullString
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO repositories (owner, repo)
		 VALUES ($1, $2)
		 ON CONFLICT (owner, repo) DO UPDATE SET updated_at = NOW()
		 RETURNING id, owner, repo, last_seen_tag, created_at, updated_at`,
		owner, repo,
	).Scan(&rep.ID, &rep.Owner, &rep.Repo, &tag, &rep.CreatedAt, &rep.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if tag.Valid {
		rep.LastSeenTag = tag.String
	}

	return &rep, nil
}

func (r *SubscriptionRepo) GetRepositoryByOwnerRepo(ctx context.Context, owner, repo string) (*domain.Repository, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var rep domain.Repository
	var tag sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, owner, repo, last_seen_tag, created_at, updated_at
		 FROM repositories WHERE owner = $1 AND repo = $2`,
		owner, repo,
	).Scan(&rep.ID, &rep.Owner, &rep.Repo, &tag, &rep.CreatedAt, &rep.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if tag.Valid {
		rep.LastSeenTag = tag.String
	}
	return &rep, nil
}

func (r *SubscriptionRepo) UpdateLastSeenTag(ctx context.Context, repoID int, tag string) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		`UPDATE repositories SET last_seen_tag = $1, updated_at = NOW() WHERE id = $2`,
		tag, repoID,
	)
	return err
}

func (r *SubscriptionRepo) GetRepositoriesWithConfirmedSubs(ctx context.Context) (repos []domain.Repository, err error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT r.id, r.owner, r.repo, r.last_seen_tag, r.created_at, r.updated_at
		 FROM repositories r
		 INNER JOIN subscriptions s ON s.repository_id = r.id
		 WHERE s.confirmed = TRUE`,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	for rows.Next() {
		var rep domain.Repository
		var tag sql.NullString
		if scanErr := rows.Scan(&rep.ID, &rep.Owner, &rep.Repo, &tag, &rep.CreatedAt, &rep.UpdatedAt); scanErr != nil {
			return nil, scanErr
		}
		if tag.Valid {
			rep.LastSeenTag = tag.String
		}
		repos = append(repos, rep)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return repos, nil
}

func (r *SubscriptionRepo) CreateSubscription(ctx context.Context, sub *domain.Subscription) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	return r.db.QueryRowContext(ctx,
		`INSERT INTO subscriptions (email, repository_id, confirmed, confirm_token, unsubscribe_token)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, created_at, updated_at`,
		sub.Email, sub.RepositoryID, sub.Confirmed, sub.ConfirmToken, sub.UnsubscribeToken,
	).Scan(&sub.ID, &sub.CreatedAt, &sub.UpdatedAt)
}

func (r *SubscriptionRepo) GetSubscriptionByEmailAndRepo(ctx context.Context, email string, repoID int) (*domain.Subscription, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var sub domain.Subscription
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, repository_id, confirmed, confirm_token, unsubscribe_token, created_at, updated_at
		 FROM subscriptions WHERE email = $1 AND repository_id = $2`,
		email, repoID,
	).Scan(&sub.ID, &sub.Email, &sub.RepositoryID, &sub.Confirmed, &sub.ConfirmToken, &sub.UnsubscribeToken, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepo) GetSubscriptionsByEmail(ctx context.Context, email string) (subs []domain.Subscription, err error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.QueryContext(ctx,
		`SELECT s.id, s.email, s.repository_id, s.confirmed, s.confirm_token, s.unsubscribe_token, s.created_at, s.updated_at,
		        r.id, r.owner, r.repo, r.last_seen_tag
		 FROM subscriptions s
		 INNER JOIN repositories r ON r.id = s.repository_id
		 WHERE s.email = $1`,
		email,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	for rows.Next() {
		var sub domain.Subscription
		var rep domain.Repository
		var tag sql.NullString
		if scanErr := rows.Scan(
			&sub.ID, &sub.Email, &sub.RepositoryID, &sub.Confirmed,
			&sub.ConfirmToken, &sub.UnsubscribeToken, &sub.CreatedAt, &sub.UpdatedAt,
			&rep.ID, &rep.Owner, &rep.Repo, &tag,
		); scanErr != nil {
			return nil, scanErr
		}
		if tag.Valid {
			rep.LastSeenTag = tag.String
		}
		sub.Repository = &rep
		subs = append(subs, sub)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *SubscriptionRepo) GetSubscriptionByConfirmToken(ctx context.Context, token string) (*domain.Subscription, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var sub domain.Subscription
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, repository_id, confirmed, confirm_token, unsubscribe_token, created_at, updated_at
		 FROM subscriptions WHERE confirm_token = $1`,
		token,
	).Scan(&sub.ID, &sub.Email, &sub.RepositoryID, &sub.Confirmed, &sub.ConfirmToken, &sub.UnsubscribeToken, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepo) GetSubscriptionByUnsubscribeToken(ctx context.Context, token string) (*domain.Subscription, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var sub domain.Subscription
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, repository_id, confirmed, confirm_token, unsubscribe_token, created_at, updated_at
		 FROM subscriptions WHERE unsubscribe_token = $1`,
		token,
	).Scan(&sub.ID, &sub.Email, &sub.RepositoryID, &sub.Confirmed, &sub.ConfirmToken, &sub.UnsubscribeToken, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *SubscriptionRepo) ConfirmSubscription(ctx context.Context, id int) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		`UPDATE subscriptions SET confirmed = TRUE, updated_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}

func (r *SubscriptionRepo) DeleteSubscription(ctx context.Context, id int) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		`DELETE FROM subscriptions WHERE id = $1`,
		id,
	)
	return err
}

func (r *SubscriptionRepo) GetConfirmedSubscriptionsByRepoID(ctx context.Context, repoID int) (subs []domain.Subscription, err error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, email, repository_id, confirmed, confirm_token, unsubscribe_token, created_at, updated_at
		 FROM subscriptions WHERE repository_id = $1 AND confirmed = TRUE`,
		repoID,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	for rows.Next() {
		var sub domain.Subscription
		if scanErr := rows.Scan(
			&sub.ID, &sub.Email, &sub.RepositoryID, &sub.Confirmed,
			&sub.ConfirmToken, &sub.UnsubscribeToken, &sub.CreatedAt, &sub.UpdatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		subs = append(subs, sub)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return subs, nil
}
