// backend/model/estudante.go
//
// 🔹 Objetivo:
// Definir os DTOs e o modelo de Estudante, com funções de saneamento e
// validação leves (sem quebrar o contrato JSON usado pelo frontend).

package model

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

// ===============================================================
// 📌 Estrutura Estudante (persistido/retorno da API)
// ---------------------------------------------------------------
// Mantém exatamente os mesmos nomes e tags JSON do seu projeto.
// ===============================================================
type Estudante struct {
	ID             int    `json:"id"`              // Identificador único do estudante
	Nome           string `json:"nome"`            // Nome completo
	CPF            string `json:"cpf"`             // CPF (documento nacional)
	Email          string `json:"email"`           // E-mail válido
	DataNascimento string `json:"data_nascimento"` // Data de nascimento (ISO 8601: YYYY-MM-DD)
	Telefone       string `json:"telefone"`        // Número de telefone
	FotoURL        string `json:"foto_url"`        // Foto de perfil do aluno
	AnoID          int    `json:"ano_id"`          // Relacionamento com tabela de anos
	TurmaID        int    `json:"turma_id"`        // Relacionamento com tabela de turmas
	UsuarioID      int    `json:"usuario_id"`      // Usuário dono do registro
}

// ===============================================================
// DTOs para criação/atualização
// ---------------------------------------------------------------
// Mantêm compatibilidade e permitem handlers mais limpos.
// ===============================================================

type EstudanteCreateRequest struct {
	Nome           string `json:"nome"`
	CPF            string `json:"cpf"`
	Email          string `json:"email"`
	DataNascimento string `json:"data_nascimento"`
	Telefone       string `json:"telefone"`
	FotoURL        string `json:"foto_url"`
	AnoID          int    `json:"ano_id"`
	TurmaID        int    `json:"turma_id"`
	UsuarioID      int    `json:"usuario_id"`
}

type EstudanteUpdateRequest struct {
	Nome           *string `json:"nome,omitempty"`
	CPF            *string `json:"cpf,omitempty"`
	Email          *string `json:"email,omitempty"`
	DataNascimento *string `json:"data_nascimento,omitempty"`
	Telefone       *string `json:"telefone,omitempty"`
	FotoURL        *string `json:"foto_url,omitempty"`
	AnoID          *int    `json:"ano_id,omitempty"`
	TurmaID        *int    `json:"turma_id,omitempty"`
	UsuarioID      *int    `json:"usuario_id,omitempty"`
}

// ===============================================================
// Saneamento e Validação
// ===============================================================

const (
	cpfDigitsRequired = 11
	dateLayoutISO     = "2006-01-02"
)

var (
	// Reutilizamos ErrNomeObrigatorio e ErrEmailInvalido do model/user.go
	ErrCPFInvalido            = errors.New("cpf inválido (precisa conter 11 dígitos)")
	ErrDataNascimentoInvalida = errors.New("data_nascimento inválida (esperado YYYY-MM-DD)")
)

// remove tudo que não for dígito
func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
}

func isValidISODate(s string) bool {
	if len(strings.TrimSpace(s)) == 0 {
		return false
	}
	_, err := time.Parse(dateLayoutISO, s)
	return err == nil
}

// --- Create: Sanitize/Validate ---

func (r *EstudanteCreateRequest) Sanitize() {
	r.Nome = strings.TrimSpace(r.Nome)
	r.CPF = digitsOnly(r.CPF)
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.DataNascimento = strings.TrimSpace(r.DataNascimento)
	r.Telefone = strings.TrimSpace(r.Telefone)
	r.FotoURL = strings.TrimSpace(r.FotoURL)
}

func (r EstudanteCreateRequest) Validate() error {
	if strings.TrimSpace(r.Nome) == "" {
		return ErrNomeObrigatorio
	}
	if len(digitsOnly(r.CPF)) != cpfDigitsRequired {
		return ErrCPFInvalido
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return ErrEmailInvalido
	}
	if !isValidISODate(r.DataNascimento) {
		return ErrDataNascimentoInvalida
	}
	return nil
}

// --- Update: Sanitize/Validate (só valida o que vier no payload) ---

func (r *EstudanteUpdateRequest) Sanitize() {
	if r.Nome != nil {
		v := strings.TrimSpace(*r.Nome)
		r.Nome = &v
	}
	if r.CPF != nil {
		v := digitsOnly(*r.CPF)
		r.CPF = &v
	}
	if r.Email != nil {
		v := strings.ToLower(strings.TrimSpace(*r.Email))
		r.Email = &v
	}
	if r.DataNascimento != nil {
		v := strings.TrimSpace(*r.DataNascimento)
		r.DataNascimento = &v
	}
	if r.Telefone != nil {
		v := strings.TrimSpace(*r.Telefone)
		r.Telefone = &v
	}
	if r.FotoURL != nil {
		v := strings.TrimSpace(*r.FotoURL)
		r.FotoURL = &v
	}
	// AnoID/TurmaID/UsuarioID: inteiros, nada a sanitizar
}

func (r EstudanteUpdateRequest) Validate() error {
	if r.Nome != nil && strings.TrimSpace(*r.Nome) == "" {
		return ErrNomeObrigatorio
	}
	if r.CPF != nil && len(digitsOnly(*r.CPF)) != cpfDigitsRequired {
		return ErrCPFInvalido
	}
	if r.Email != nil {
		if _, err := mail.ParseAddress(*r.Email); err != nil {
			return ErrEmailInvalido
		}
	}
	if r.DataNascimento != nil && !isValidISODate(*r.DataNascimento) {
		return ErrDataNascimentoInvalida
	}
	return nil
}

// ===============================================================
// Helpers de conversão (opcional)
// ---------------------------------------------------------------
// Úteis quando o handler quiser transformar o DTO em model.Estudante.
// ===============================================================

func (r EstudanteCreateRequest) ToModel() Estudante {
	return Estudante{
		Nome:           r.Nome,
		CPF:            r.CPF,
		Email:          r.Email,
		DataNascimento: r.DataNascimento,
		Telefone:       r.Telefone,
		FotoURL:        r.FotoURL,
		AnoID:          r.AnoID,
		TurmaID:        r.TurmaID,
		UsuarioID:      r.UsuarioID,
	}
}

func (u EstudanteUpdateRequest) ApplyTo(e *Estudante) {
	if u.Nome != nil {
		e.Nome = *u.Nome
	}
	if u.CPF != nil {
		e.CPF = *u.CPF
	}
	if u.Email != nil {
		e.Email = *u.Email
	}
	if u.DataNascimento != nil {
		e.DataNascimento = *u.DataNascimento
	}
	if u.Telefone != nil {
		e.Telefone = *u.Telefone
	}
	if u.FotoURL != nil {
		e.FotoURL = *u.FotoURL
	}
	if u.AnoID != nil {
		e.AnoID = *u.AnoID
	}
	if u.TurmaID != nil {
		e.TurmaID = *u.TurmaID
	}
	if u.UsuarioID != nil {
		e.UsuarioID = *u.UsuarioID
	}
}
