package models

// Admin "inherits" from User via embedding. The distinguishing field is Role.
// By convention, Role is "admin" for admins and defaults to "end user" for regular users.
type Admin struct {
    User
}

// NewAdmin creates an admin model with Role preset to "admin".
func NewAdmin(username string) *Admin {
    return &Admin{User: User{Username: username, Role: "admin"}}
}
