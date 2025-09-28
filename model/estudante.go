/*
/// Projeto: Tecmise
/// Arquivo: backend/model/estudante.go
/// Responsabilidade: Definir modelo e DTOs de Estudante com rotinas de saneamento e validação leves (compatíveis com o contrato JSON do frontend).
/// Dependências principais: time (parse ISO date), net/mail (validação básica de e-mail), unicode/strings (saneamento).
/// Pontos de atenção:
/// - CPF: valida apenas quantidade de dígitos (11). Não executa validação de dígitos verificadores (DV).
/// - Data de nascimento: aceita formato ISO (YYYY-MM-DD) via time.Parse; não verifica coerência (ex.: datas futuras).
/// - E-mail: usa mail.ParseAddress (permissivo) e não restringe provedores.
/// - Referências de erro: ErrNomeObrigatorio e ErrEmailInvalido são esperadas em model/user.go.
/// - Sanitize/Validate não normalizam telefone (apenas trim); regras de formatação podem variar por região.
/// - Tipos Update usam ponteiros para diferenciar "campo não enviado" de "limpar para string vazia".
*/

//
// backend/model/estudante.go
//
// 🔹 Objetivo:
// Definir os DTOs e o modelo de Estudante, com funções de saneamento e
// validação leves (sem quebrar o contrato JSON usado pelo frontend).
//

package model

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

/// ============ Tipos & Interfaces ============

// ===============================================================
// 📌 Estrutura Estudante (persistido/retorno da API)
// ---------------------------------------------------------------
// Mantém exatamente os mesmos nomes e tags JSON do seu projeto.
// ===============================================================

// Estudante representa o registro persistido e também o payload de resposta
// exposto pela API. As tags JSON são contratuais com o frontend.
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

/// ============ DTOs (criação/atualização) ============

// ===============================================================
// DTOs para criação/atualização
// ---------------------------------------------------------------
// Mantêm compatibilidade e permitem handlers mais limpos.
// ===============================================================

// EstudanteCreateRequest define o payload esperado para criação de estudante.
// Use Sanitize() antes de Validate() para normalizar os campos.
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

// EstudanteUpdateRequest define um payload parcial de atualização.
// Campos como ponteiros permitem diferenciar ausência de campo (nil)
// de intenção de esvaziar (ex.: string vazia).
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

/// ============ Configurações & Constantes ============

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

/// ============ Funções Internas (helpers) ============

// digitsOnly remove todos os caracteres não numéricos de uma string.
func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) {
			return r
		}
		return -1
	}, s)
}

// isValidISODate verifica se a string representa uma data válida no layout ISO (YYYY-MM-DD).
func isValidISODate(s string) bool {
	if len(strings.TrimSpace(s)) == 0 {
		return false
	}
	_, err := time.Parse(dateLayoutISO, s)
	return err == nil
}

/// ============ Funções Públicas ============

// --- Create: Sanitize/Validate ---

// Sanitize padroniza espaços e caixa dos campos de criação:
// - Trim em Nome, DataNascimento, Telefone, FotoURL
// - Apenas dígitos em CPF
// - E-mail para minúsculas e trim
func (r *EstudanteCreateRequest) Sanitize() {
	r.Nome = strings.TrimSpace(r.Nome)
	r.CPF = digitsOnly(r.CPF)
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	r.DataNascimento = strings.TrimSpace(r.DataNascimento)
	r.Telefone = strings.TrimSpace(r.Telefone)
	r.FotoURL = strings.TrimSpace(r.FotoURL)
}

// Validate executa verificações mínimas de negócio para criação:
// - Nome obrigatório
// - CPF com 11 dígitos
// - E-mail válido (mail.ParseAddress)
// - Data de nascimento em formato ISO
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

// Sanitize normaliza apenas os campos presentes (não-nil) no payload de atualização.
// Observação: IDs (AnoID/TurmaID/UsuarioID) são inteiros e não exigem saneamento textual.
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

// Validate verifica os campos informados (não-nil) no payload parcial de update.
// Mantém as mesmas regras do create onde aplicável.
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

/// ============ Helpers de conversão (opcional) ============

// ===============================================================
// Helpers de conversão (opcional)
// ---------------------------------------------------------------
// Úteis quando o handler quiser transformar o DTO em model.Estudante.
// ===============================================================

// ToModel converte um EstudanteCreateRequest para a entidade Estudante.
// Não atribui ID (geralmente é responsabilidade da camada de persistência).
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

// ApplyTo aplica os campos presentes (não-nil) de um EstudanteUpdateRequest
// sobre uma instância existente de Estudante (mutação in-place).
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

// TODO: considerar regras adicionais de negócio (ex.: impedir datas futuras, validar formato E.164 para telefone, validar DV do CPF)
// TODO: se necessário, internacionalizar mensagens de erro (i18n) mantendo contrato da API
