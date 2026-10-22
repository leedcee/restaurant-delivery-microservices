package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"restaurant-delivery-system/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PartnerRole string

const (
	PartnerRoleOwner    PartnerRole = "owner"
	PartnerRoleManager  PartnerRole = "manager"
	PartnerRoleKitchen  PartnerRole = "kitchen"
	PartnerRoleOperator PartnerRole = "operator"
)

type PartnerPermission string

const (
	PartnerReadOrders       PartnerPermission = "orders:read"
	PartnerUpdateOrder      PartnerPermission = "orders:update"
	PartnerReadMenu         PartnerPermission = "menu:read"
	PartnerUpdateMenu       PartnerPermission = "menu:update"
	PartnerManageFailures   PartnerPermission = "failures:manage"
	PartnerManageCredential PartnerPermission = "credentials:manage"
	PartnerReadAudit        PartnerPermission = "audit:read"
)

type PartnerIdentity struct {
	CredentialID uuid.UUID
	RestaurantID uuid.UUID
	Role         PartnerRole
}

func (identity PartnerIdentity) Can(permission PartnerPermission) bool {
	switch identity.Role {
	case PartnerRoleOwner:
		return true
	case PartnerRoleManager:
		return permission != PartnerManageCredential && permission != PartnerReadAudit
	case PartnerRoleKitchen:
		return permission == PartnerReadOrders || permission == PartnerUpdateOrder || permission == PartnerReadMenu
	case PartnerRoleOperator:
		return permission == PartnerReadOrders || permission == PartnerReadMenu || permission == PartnerManageFailures || permission == PartnerReadAudit
	default:
		return false
	}
}

type PartnerCredential struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	Role       PartnerRole `json:"role"`
	CreatedAt  time.Time   `json:"createdAt"`
	ExpiresAt  *time.Time  `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time  `json:"revokedAt,omitempty"`
	LastUsedAt *time.Time  `json:"lastUsedAt,omitempty"`
}

type PartnerAuditEntry struct {
	Identity     PartnerIdentity
	Action       string
	ResourceType string
	ResourceID   string
	RequestID    string
	TraceID      string
	Reason       string
	Metadata     map[string]any
}

type PartnerAuditRecord struct {
	ID             int64          `json:"id"`
	CredentialID   *uuid.UUID     `json:"credentialId,omitempty"`
	CredentialName *string        `json:"credentialName,omitempty"`
	Role           *PartnerRole   `json:"role,omitempty"`
	Action         string         `json:"action"`
	ResourceType   string         `json:"resourceType"`
	ResourceID     *string        `json:"resourceId,omitempty"`
	RequestID      *string        `json:"requestId,omitempty"`
	TraceID        *string        `json:"traceId,omitempty"`
	Reason         *string        `json:"reason,omitempty"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"createdAt"`
}

func (q *Queries) AuthenticatePartner(ctx context.Context, apiKey string) (PartnerIdentity, error) {
	if strings.TrimSpace(apiKey) == "" {
		return PartnerIdentity{}, domain.ErrUnauthorized
	}
	hash := partnerSecretHash(apiKey)
	var identity PartnerIdentity
	err := q.db.QueryRow(ctx, `UPDATE partner_credentials
		SET last_used_at = CASE
			WHEN last_used_at IS NULL OR last_used_at < now() - interval '1 minute' THEN now()
			ELSE last_used_at
		END
		WHERE secret_hash = $1 AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > now())
		RETURNING id, restaurant_id, role`, hash).
		Scan(&identity.CredentialID, &identity.RestaurantID, &identity.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return PartnerIdentity{}, domain.ErrUnauthorized
	}
	if err != nil {
		return PartnerIdentity{}, fmt.Errorf("authenticate partner: %w", err)
	}
	return identity, nil
}

func (q *Queries) ListPartnerCredentials(ctx context.Context, restaurantID uuid.UUID) ([]PartnerCredential, error) {
	rows, err := q.db.Query(ctx, `SELECT id, name, role, created_at, expires_at, revoked_at, last_used_at
		FROM partner_credentials WHERE restaurant_id = $1 ORDER BY created_at DESC`, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("list partner credentials: %w", err)
	}
	defer rows.Close()
	items := make([]PartnerCredential, 0)
	for rows.Next() {
		var item PartnerCredential
		if err := rows.Scan(&item.ID, &item.Name, &item.Role, &item.CreatedAt, &item.ExpiresAt, &item.RevokedAt, &item.LastUsedAt); err != nil {
			return nil, fmt.Errorf("scan partner credential: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (q *Queries) ListPartnerAudit(ctx context.Context, restaurantID uuid.UUID) ([]PartnerAuditRecord, error) {
	rows, err := q.db.Query(ctx, `SELECT a.id, a.credential_id, c.name, c.role, a.action,
		a.resource_type, a.resource_id, a.request_id, a.trace_id, a.reason, a.metadata, a.created_at
		FROM partner_audit_log a
		LEFT JOIN partner_credentials c ON c.id = a.credential_id
		WHERE a.restaurant_id = $1 ORDER BY a.created_at DESC, a.id DESC LIMIT 200`, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("list partner audit: %w", err)
	}
	defer rows.Close()
	items := make([]PartnerAuditRecord, 0)
	for rows.Next() {
		var item PartnerAuditRecord
		var credentialID *uuid.UUID
		var credentialName, roleText *string
		var metadata []byte
		if err := rows.Scan(&item.ID, &credentialID, &credentialName, &roleText, &item.Action,
			&item.ResourceType, &item.ResourceID, &item.RequestID, &item.TraceID, &item.Reason,
			&metadata, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan partner audit: %w", err)
		}
		item.CredentialID = credentialID
		item.CredentialName = credentialName
		if roleText != nil {
			role := PartnerRole(*roleText)
			item.Role = &role
		}
		if err := json.Unmarshal(metadata, &item.Metadata); err != nil {
			return nil, fmt.Errorf("decode partner audit metadata: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (q *Queries) CreatePartnerCredential(ctx context.Context, actor PartnerIdentity, name, plainSecret string, role PartnerRole, expiresAt *time.Time, audit PartnerAuditEntry) (PartnerCredential, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 || !validPartnerRole(role) {
		return PartnerCredential{}, fmt.Errorf("%w: invalid credential name or role", domain.ErrValidation)
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return PartnerCredential{}, fmt.Errorf("%w: expiration must be in the future", domain.ErrValidation)
	}
	tx, err := q.db.Begin(ctx)
	if err != nil {
		return PartnerCredential{}, fmt.Errorf("begin credential transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var created PartnerCredential
	err = tx.QueryRow(ctx, `INSERT INTO partner_credentials (restaurant_id, name, secret_hash, role, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, role, created_at, expires_at, revoked_at, last_used_at`,
		actor.RestaurantID, name, partnerSecretHash(plainSecret), role, expiresAt).
		Scan(&created.ID, &created.Name, &created.Role, &created.CreatedAt, &created.ExpiresAt, &created.RevokedAt, &created.LastUsedAt)
	if err != nil {
		return PartnerCredential{}, fmt.Errorf("create partner credential: %w", err)
	}
	audit.Identity = actor
	audit.Action = "credential.created"
	audit.ResourceType = "partner_credential"
	audit.ResourceID = created.ID.String()
	audit.Metadata = map[string]any{"name": created.Name, "role": created.Role, "expiresAt": created.ExpiresAt}
	if err := insertPartnerAudit(ctx, tx, audit); err != nil {
		return PartnerCredential{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PartnerCredential{}, fmt.Errorf("commit credential creation: %w", err)
	}
	return created, nil
}

func (q *Queries) RevokePartnerCredential(ctx context.Context, actor PartnerIdentity, credentialID uuid.UUID, audit PartnerAuditEntry) error {
	if actor.CredentialID == credentialID {
		return fmt.Errorf("%w: use a replacement owner credential to revoke the current key", domain.ErrConflict)
	}
	tx, err := q.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin credential revocation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var role PartnerRole
	err = tx.QueryRow(ctx, `SELECT role FROM partner_credentials
		WHERE id = $1 AND restaurant_id = $2 AND revoked_at IS NULL FOR UPDATE`, credentialID, actor.RestaurantID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load partner credential: %w", err)
	}
	if role == PartnerRoleOwner {
		var activeOwners int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM partner_credentials
			WHERE restaurant_id = $1 AND role = 'owner' AND revoked_at IS NULL
			  AND (expires_at IS NULL OR expires_at > now())`, actor.RestaurantID).Scan(&activeOwners); err != nil {
			return fmt.Errorf("count active owner credentials: %w", err)
		}
		if activeOwners <= 1 {
			return fmt.Errorf("%w: the restaurant must retain an active owner credential", domain.ErrConflict)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE partner_credentials SET revoked_at = now() WHERE id = $1`, credentialID); err != nil {
		return fmt.Errorf("revoke partner credential: %w", err)
	}
	audit.Identity = actor
	audit.Action = "credential.revoked"
	audit.ResourceType = "partner_credential"
	audit.ResourceID = credentialID.String()
	audit.Metadata = map[string]any{"role": role}
	if err := insertPartnerAudit(ctx, tx, audit); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit credential revocation: %w", err)
	}
	return nil
}

func (q *Queries) RecordPartnerAudit(ctx context.Context, entry PartnerAuditEntry) error {
	return insertPartnerAudit(ctx, q.db, entry)
}

type auditExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertPartnerAudit(ctx context.Context, executor auditExecutor, entry PartnerAuditEntry) error {
	if entry.Metadata == nil {
		entry.Metadata = map[string]any{}
	}
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("marshal partner audit metadata: %w", err)
	}
	_, err = executor.Exec(ctx, `INSERT INTO partner_audit_log
		(restaurant_id, credential_id, action, resource_type, resource_id, request_id, trace_id, reason, metadata)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)`,
		entry.Identity.RestaurantID, entry.Identity.CredentialID, entry.Action, entry.ResourceType,
		entry.ResourceID, entry.RequestID, entry.TraceID, entry.Reason, metadata)
	if err != nil {
		return fmt.Errorf("record partner audit: %w", err)
	}
	return nil
}

func partnerSecretHash(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(hash[:])
}

func validPartnerRole(role PartnerRole) bool {
	return role == PartnerRoleOwner || role == PartnerRoleManager || role == PartnerRoleKitchen || role == PartnerRoleOperator
}
