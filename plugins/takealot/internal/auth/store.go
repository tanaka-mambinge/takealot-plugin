package auth

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "takealot-cli"
	keyringAccount = "default"
)

var ErrNotFound = errors.New("takealot authentication session not found")

type Backend interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
	Delete(service, account string) error
}

type keyringBackend struct{}

func (keyringBackend) Get(service, account string) (string, error) {
	return keyring.Get(service, account)
}
func (keyringBackend) Set(service, account, secret string) error {
	return keyring.Set(service, account, secret)
}
func (keyringBackend) Delete(service, account string) error { return keyring.Delete(service, account) }

type Store struct {
	backend Backend
	service string
	account string
}

func NewStore() *Store {
	return NewStoreWithBackend(keyringBackend{})
}

func NewStoreWithBackend(backend Backend) *Store {
	if backend == nil {
		backend = keyringBackend{}
	}
	return &Store{backend: backend, service: keyringService, account: keyringAccount}
}

type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain,omitempty"`
	Path     string    `json:"path,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HttpOnly bool      `json:"http_only,omitempty"`
}

// Session contains the mobile API credentials. It is only serialized into the OS keyring.
type Session struct {
	JWT                   string    `json:"jwt"`
	IDToken               string    `json:"id_token,omitempty"`
	RefreshToken          string    `json:"refresh_token"`
	CSRFToken             string    `json:"csrf_token,omitempty"`
	TrackingID            string    `json:"tracking_id,omitempty"`
	CustomerID            string    `json:"customer_id"`
	AccessKey             string    `json:"access_key,omitempty"`
	PrivateKey            string    `json:"private_key,omitempty"`
	DID                   string    `json:"did,omitempty"`
	JWTExpiresAt          time.Time `json:"jwt_expires_at,omitempty"`
	IDTokenExpiresAt      time.Time `json:"id_token_expires_at,omitempty"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at,omitempty"`
	Cookies               []Cookie  `json:"cookies,omitempty"`
}

func (s *Store) Save(session Session) error {
	if session.JWT == "" || session.RefreshToken == "" || session.CustomerID == "" {
		return errors.New("takealot login response did not contain a complete session")
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return s.backend.Set(s.service, s.account, string(encoded))
}

func (s *Store) Load() (Session, error) {
	secret, err := s.backend.Get(s.service, s.account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Session{}, ErrNotFound
		}
		return Session{}, errors.New("read Takealot session from OS keyring: " + err.Error())
	}
	var session Session
	if err := json.Unmarshal([]byte(secret), &session); err != nil {
		return Session{}, errors.New("Takealot session in OS keyring is invalid")
	}
	if session.JWT == "" || session.RefreshToken == "" || session.CustomerID == "" {
		return Session{}, errors.New("Takealot session in OS keyring is incomplete; log in again")
	}
	return session, nil
}

func (s *Store) Delete() error {
	err := s.backend.Delete(s.service, s.account)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
