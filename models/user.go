package models

import "github.com/golang-jwt/jwt/v5"

// Role is the set of roles a user can have.
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// User represents an application user stored in memory.
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Role         Role   `json:"role"`
}

// Claims is the JWT payload embedded in every token issued by this service.
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	jwt.RegisteredClaims
}
