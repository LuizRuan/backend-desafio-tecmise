// backend/model/user.go
package model

// RegisterRequest representa o payload de cadastro de usuário
type RegisterRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// User representa os dados do usuário retornados pela API
type User struct {
	ID            int    `json:"id"`
	Nome          string `json:"nome"`            // Nome do usuário
	Email         string `json:"email"`           // E-mail
	Senha         string `json:"senha,omitempty"` // Senha omitida no retorno
	FotoUrl       string `json:"fotoUrl"`         // URL da foto de perfil
	TutorialVisto bool   `json:"tutorial_visto"`  // Indica se o tutorial já foi visto
}
