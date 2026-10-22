package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"imaskop/internal/model"
	"imaskop/internal/repository"
)

type Error struct {
	Status int
	Code   string
}

func (e *Error) Error() string   { return e.Code }
func Bad(code string) error      { return &Error{400, code} }
func Forbidden() error           { return &Error{403, "forbidden"} }
func Conflict(code string) error { return &Error{409, code} }

type Service struct {
	Store  *repository.Store
	Secret []byte
}
type Claims struct {
	Subject string `json:"sub"`
	Expires int64  `json:"exp"`
}
type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

func (s *Service) CreateUser(ctx context.Context, in Credentials, role model.UserRole) (model.User, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	addr, err := mail.ParseAddress(in.Email)
	if err != nil || addr.Address != in.Email || len(in.Email) > 255 || len(in.Name) == 0 || len(in.Name) > 150 || len(in.Password) < 12 || len(in.Password) > 72 {
		return model.User{}, Bad("invalid_credentials_fields")
	}
	if role != model.RoleCompany && role != model.RoleCustomer && role != model.RoleAdmin {
		return model.User{}, Bad("invalid_role")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}
	var u model.User
	err = s.Store.DB.QueryRowContext(ctx, "INSERT INTO users(email,password_hash,name,role) VALUES($1,$2,$3,$4) RETURNING id,email,name,role,created_at,updated_at", in.Email, string(hash), in.Name, role).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

func (s *Service) Login(ctx context.Context, in Credentials) (map[string]any, error) {
	if len(in.Email) > 255 || len(in.Password) > 72 {
		return nil, &Error{401, "invalid_credentials"}
	}
	u, err := repository.User(ctx, s.Store.DB, "lower(email)=$1", strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil {
		if !repository.IsMissing(err) {
			return nil, err
		}
		bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO5nR9tc7.LbTy0l0XP7PXhCNV7gIHr5e"), []byte(in.Password))
		return nil, &Error{401, "invalid_credentials"}
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)) != nil {
		return nil, &Error{401, "invalid_credentials"}
	}
	exp := time.Now().Add(8 * time.Hour)
	payload, _ := json.Marshal(Claims{u.ID.String(), exp.Unix()})
	p := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(p))
	token := p + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return map[string]any{"access_token": token, "token_type": "Bearer", "expires_at": exp.UTC(), "user": u}, nil
}
func (s *Service) Authenticate(ctx context.Context, token string) (model.User, error) {
	invalid := &Error{401, "invalid_token"}
	if len(token) > 2048 {
		return model.User{}, invalid
	}
	p, sig, ok := strings.Cut(token, ".")
	if !ok {
		return model.User{}, invalid
	}
	b, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return model.User{}, invalid
	}
	mac := hmac.New(sha256.New, s.Secret)
	mac.Write([]byte(p))
	if !hmac.Equal(b, mac.Sum(nil)) {
		return model.User{}, invalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return model.User{}, invalid
	}
	var c Claims
	if json.Unmarshal(payload, &c) != nil || c.Expires <= time.Now().Unix() {
		return model.User{}, invalid
	}
	if _, err = uuid.Parse(c.Subject); err != nil {
		return model.User{}, invalid
	}
	u, err := repository.User(ctx, s.Store.DB, "id=$1", c.Subject)
	if repository.IsMissing(err) {
		return u, invalid
	}
	return u, err
}
func Role(u model.User, roles ...model.UserRole) error {
	for _, role := range roles {
		if u.Role == role {
			return nil
		}
	}
	return Forbidden()
}
