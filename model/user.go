/*
/// Projeto: Tecmise
/// Arquivo: backend/model/user.go
/// Responsabilidade: DTOs e entidade de Usuário (registro, login, atualização de perfil, flags de tutorial).
/// Dependências principais: errors, net/mail (validação básica de e-mail), strings.
/// Pontos de atenção:
/// - MinPasswordLen=6 enquanto o frontend (login.vue) valida senha mínima 8 para login (possível divergência de UX/contrato).
/// - Convenção de JSON: mistura camelCase (`fotoUrl`) e snake_case (`tutorial_visto`) por compatibilidade com o frontend.
/// - mail.ParseAddress é permissivo; não valida domínio/entregabilidade.
/// - Sanitize/Validate são leves; regras específicas de negócio devem ficar no handler/camada de serviço.
*/

// backend/model/user.go
package model

import (
	"errors"
	"net/mail"
	"strings"
)

/// ============ Tipos & Interfaces ============

/*
===========================================
📌 Estrutura RegisterRequest
-------------------------------------------
- Payload para cadastro de um novo usuário.
- Mantém compatibilidade com os handlers atuais.
===========================================
*/
// RegisterRequest define os campos esperados para cadastro de usuário.
type RegisterRequest struct {
	Nome  string `json:"nome"`  // Nome do usuário a ser cadastrado
	Email string `json:"email"` // E-mail único usado no login
	Senha string `json:"senha"` // Senha em texto puro no payload
}

/// ============ Configurações & Constantes ============

// Regras básicas (podem ser ajustadas via handler, se preferir)
const MinPasswordLen = 6

var (
	ErrNomeObrigatorio = errors.New("nome é obrigatório")
	ErrEmailInvalido   = errors.New("email inválido")
	ErrSenhaCurta      = errors.New("senha muito curta")
)

/// ============ Funções Públicas ============

// Sanitize normaliza campos de entrada (trim e e-mail minúsculo).
// Efeitos colaterais: muta o próprio receiver.
func (r *RegisterRequest) Sanitize() {
	r.Nome = strings.TrimSpace(r.Nome)
	r.Email = strings.TrimSpace(strings.ToLower(r.Email))
}

// Validate aplica validações simples para cadastro.
// Regras: nome não vazio, e-mail válido por mail.ParseAddress e senha com tamanho mínimo.
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
// LoginRequest representa o payload de autenticação tradicional (email/senha).
type LoginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// Sanitize para LoginRequest: trim + lowercase no e-mail.
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
// UpdatePerfilRequest traz campos opcionais para atualização de perfil do usuário.
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
// TutorialUpdateRequest encapsula a alteração do flag de tutorial.
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
// User é a entidade persistida e resposta padrão de usuário.
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
// UserPublic é a projeção segura do usuário, sem expor senha.
type UserPublic struct {
	ID            int    `json:"id"`
	Nome          string `json:"nome"`
	Email         string `json:"email"`
	FotoURL       string `json:"fotoUrl"`
	TutorialVisto bool   `json:"tutorial_visto"`
}

// Public projeta um User para UserPublic (sem senha).
// Não altera o receiver; apenas retorna uma cópia convertida.
func (u User) Public() UserPublic {
	return UserPublic{
		ID:            u.ID,
		Nome:          u.Nome,
		Email:         u.Email,
		FotoURL:       u.FotoURL,
		TutorialVisto: u.TutorialVisto,
	}
}

// TODO: avaliar alinhamento de MinPasswordLen com validações do frontend (ex.: 8+ chars no login/register UI)
// TODO: padronizar convenção JSON (camelCase vs snake_case) quando possível, mantendo compatibilidade retroativa
