package model

import (
	"time"
)

type User struct {
	ID           string    `structs:"id" json:"id" orm:"column(id)"`
	UserName     string    `structs:"user_name,omitempty" json:"userName,omitempty"`
	Name         string    `structs:"name,omitempty" json:"name,omitempty"`
	Email        string    `structs:"email,omitempty" json:"email,omitempty"`
	IsAdmin      bool      `structs:"is_admin,omitempty" json:"isAdmin,omitempty"`
	LastLoginAt  time.Time `structs:"last_login_at,omitempty" json:"lastLoginAt,omitempty"`
	LastAccessAt time.Time `structs:"last_access_at,omitempty" json:"lastAccessAt,omitempty"`
	CreatedAt    time.Time `structs:"created_at,omitempty" json:"createdAt,omitempty"`
	UpdatedAt    time.Time `structs:"updated_at,omitempty" json:"updatedAt,omitempty"`

	SubscriptionExpDate time.Time `structs:"subscription_exp_date,omitempty" json:"SubscriptionExpDate,omitempty"`

	// This is only available on the backend, and it is never sent over the wire
	Password string `structs:"-" json:"-"`
	// This is used to set or change a password when calling Put. If it is empty, the password is not changed.
	// It is received from the UI with the name "password"
	NewPassword string `structs:"password,omitempty" json:"password,omitempty"`
	// If changing the password, this is also required
	CurrentPassword string `structs:"current_password,omitempty" json:"currentPassword,omitempty"`
}

type Users []User

type UserRepository interface {
	CountAll(...QueryOptions) (int64, error)
	Get(id string) (*User, error)
	Put(*User) error
	UpdateLastLoginAt(id string) error
	UpdateLastAccessAt(id string) error
	FindFirstAdmin() (*User, error)
	// FindByUsername must be case-insensitive
	FindByUsername(username string) (*User, error)
	FindByEmail(email string) (*User, error)
	// FindByUsernameWithPassword is the same as above, but also returns the decrypted password
	FindByUsernameWithPassword(username string) (*User, error)
}

// var (
// 	DemoUser = User{
// 		ID:       uuid.NewString(),
// 		UserName: "DemoUser",
// 		Name:     "demo",
// 		Email:    "",
// 		IsAdmin:  false,
// 	}
// )
