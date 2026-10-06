package identity

import "time"

type Claims struct {
	Subject string   `json:"sub"`
	Role    string   `json:"role"`
	Roles   []string `json:"roles"`
	Expires int64    `json:"exp"`
	Issued  int64    `json:"iat"`
}

func (c Claims) HasAdminRole() bool {
	if IsAdminRole(c.Role) {
		return true
	}
	for _, role := range c.Roles {
		if IsAdminRole(role) {
			return true
		}
	}
	return false
}

func (c Claims) Expired(now time.Time) bool {
	if c.Expires <= 0 {
		return true
	}
	return now.Unix() >= c.Expires
}
