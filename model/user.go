// backend/model/user.go
package model

import (
	"errors"
	"net/mail"
	"strings"
)

/*
===========================================
📌 Estrutura RegisterRequest
-------------------------------------------
- Payload para cadastro de um novo usuário.
- Mantém compatibilidade com os handlers atuais.
===========================================
*/
type RegisterRequest struct {
	Nome  string `json:"nome"`  // Nome do usuário a ser cadastrado
	Email string `json:"email"` // E-mail único usado no login
	Senha string `json:"senha"` // Senha em texto puro no payload
}

// Regras básicas (podem ser ajustadas via handler, se preferir)
const MinPasswordLen = 6

var (
	ErrNomeObrigatorio = errors.New("nome é obrigatório")
	ErrEmailInvalido   = errors.New("email inválido")
	ErrSenhaCurta      = errors.New("senha muito curta")
)

// Sanitize normaliza campos de entrada (trim e e-mail minúsculo).
func (r *RegisterRequest) Sanitize() {
	r.Nome = strings.TrimSpace(r.Nome)
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))
}

// Validate aplica validações simples para cadastro.
func (r RegisterRequest) Validate() error {
	if strings.TrimSpace(r.Nome) == "" {
		return ErrNomeObrigatorio
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return ErrEmailInvalido
	}
	if len(r.Senha) < MinPasswordLen {
		return ErrSenhaCurta
	}
	return nil
}

/*
===========================================
📌 Estrutura LoginRequest (opcional)
-------------------------------------------
  - Útil para o endpoint de login. Acrescentada
    aqui para centralizar DTOs do usuário.
  - Não quebra nada existente (uso opcional).

===========================================
*/
type LoginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

func (l *LoginRequest) Sanitize() {
	l.Email = strings.TrimSpace(strings.ToLower(l.Email))
}

/*
===========================================
📌 Estrutura UpdatePerfilRequest (opcional)
-------------------------------------------
  - Payload para atualização do perfil.
  - Campos como ponteiros permitem "opcionalidade"
    real (diferencia ausente de vazio).
  - Pode ser usada em handlers atuais sem
    afetar o modelo User.

===========================================
*/
type UpdatePerfilRequest struct {
	Nome    *string `json:"nome,omitempty"`
	FotoURL *string `json:"fotoUrl,omitempty"`
	Senha   *string `json:"senha,omitempty"` // opcional
}

/*
===========================================
📌 Estrutura TutorialUpdateRequest (opcional)
-------------------------------------------
- Para a rota: PUT /api/usuario/{id}/tutorial
===========================================
*/
type TutorialUpdateRequest struct {
	TutorialVisto bool `json:"tutorial_visto"`
}

/*
===========================================
📌 Estrutura User
-------------------------------------------
  - Representa os dados persistidos/retornados
    do usuário. Mantém o campo Senha com
    `omitempty` para evitar vazamento acidental
    quando estiver vazio.

===========================================
*/
type User struct {
	ID            int    `json:"id"`              // Identificador único no banco
	Nome          string `json:"nome"`            // Nome do usuário
	Email         string `json:"email"`           // E-mail de login
	Senha         string `json:"senha,omitempty"` // Senha omitida no retorno
	FotoURL       string `json:"fotoUrl"`         // URL da foto de perfil do usuário
	TutorialVisto bool   `json:"tutorial_visto"`  // Flag: indica se o tutorial já foi visto
}

/*
===========================================
📌 Estrutura UserPublic
-------------------------------------------
  - Versão "segura" para respostas: nunca inclui
    a senha. Handlers podem preferir retornar
    este tipo.

===========================================
*/
type UserPublic struct {
	ID            int    `json:"id"`
	Nome          string `json:"nome"`
	Email         string `json:"email"`
	FotoURL       string `json:"fotoUrl"`
	TutorialVisto bool   `json:"tutorial_visto"`
}

// Public projeta um User para UserPublic (sem senha).
func (u User) Public() UserPublic {
	return UserPublic{
		ID:            u.ID,
		Nome:          u.Nome,
		Email:         u.Email,
		FotoURL:       u.FotoURL,
		TutorialVisto: u.TutorialVisto,
	}
}
