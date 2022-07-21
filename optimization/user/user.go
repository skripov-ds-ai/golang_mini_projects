package user

type User struct {
	Browsers []string `json:"browsers,omitempty"`
	Name     string   `json:"name,omitempty"`
	Email    string   `json:"email,omitempty"`
}
