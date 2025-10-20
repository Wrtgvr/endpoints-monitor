package domain

import "errors"

const (
	KeyTypeReadOnly string = "read_only"
	KeyTypeAdmin    string = "admin"
)

type APIKey struct {
	Key       string
	Type      string
	ProjectID string
	CreatedAt string
}

func (k *APIKey) Validate() error {
	if k.Key == "" {
		return errors.New("key cannot be empty")
	}
	if k.ProjectID == "" {
		return errors.New("project ID cannot be empty")
	}
	if k.Type != KeyTypeReadOnly && k.Type != KeyTypeAdmin {
		return errors.New("invalid key type")
	}
	return nil
}
