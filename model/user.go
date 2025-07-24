package model

type RegisterRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

type User struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Senha string `json:"senha,omitempty"`
}
