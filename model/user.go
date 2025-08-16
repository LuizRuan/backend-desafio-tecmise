// backend/model/user.go
package model

/*
===========================================
📌 Estrutura RegisterRequest
-------------------------------------------
- Representa o payload recebido no cadastro
  de um novo usuário do sistema.

Campos esperados no JSON:
- Nome  (string) → Nome completo do usuário
- Email (string) → E-mail de login (único)
- Senha (string) → Senha em texto puro
===========================================
*/
type RegisterRequest struct {
	Nome  string `json:"nome"`  // Nome do usuário a ser cadastrado
	Email string `json:"email"` // E-mail único usado no login
	Senha string `json:"senha"` // Senha em texto puro no payload
}

/*
===========================================
📌 Estrutura User
-------------------------------------------
- Representa os dados de um usuário já
  cadastrado no banco e retornado pela API.

Campos retornados no JSON:
- ID            (int)    → Identificador único
- Nome          (string) → Nome do usuário
- Email         (string) → Endereço de e-mail
- Senha         (string) → Senha omitida no retorno
- FotoUrl       (string) → URL da foto de perfil
- TutorialVisto (bool)   → Se o tutorial já foi concluído
===========================================
*/
type User struct {
	ID            int    `json:"id"`              // Identificador único no banco
	Nome          string `json:"nome"`            // Nome do usuário
	Email         string `json:"email"`           // E-mail de login
	Senha         string `json:"senha,omitempty"` // Senha omitida no retorno
	FotoUrl       string `json:"fotoUrl"`         // URL da foto de perfil do usuário
	TutorialVisto bool   `json:"tutorial_visto"`  // Flag: indica se o tutorial já foi visto
}
