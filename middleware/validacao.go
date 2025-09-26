// backend/middleware/validacao.go
//
// 🔹 Objetivo:
// Middlewares de validação/saneamento para cadastro, login e email do estudante.
// Mantém comportamento (status 400 e mensagens em texto) e reduz duplicação.
// - Reutiliza DTOs e regras do package model (RegisterRequest, LoginRequest, MinPasswordLen)
// - Usa net/mail para validação de e-mail (mais robusto que regex)
// - Reinsere o corpo normalizado sem conversões desnecessárias

package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"strings"

	"backend/model"
)

// Limite de corpo lido (proteção básica contra payloads gigantes)
const maxBodySize = 1 << 20 // 1 MiB

// ------------------------ helpers ------------------------

func normalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	if email == "" {
		return "", http.ErrNoLocation // só para sinalizar vazio; tratamos fora
	}
	// Não aceitamos espaços internos
	if strings.Contains(email, " ") {
		return "", http.ErrUseLastResponse // marcador genérico
	}
	// Validação RFC-ish
	if _, err := mail.ParseAddress(email); err != nil {
		return "", err
	}
	// Normalização comum: minúsculas
	return strings.ToLower(email), nil
}

// ---------------------- Middlewares ----------------------

// ValidarCadastroMiddleware valida o payload de cadastro de usuário.
// Regras: nome ≥ 2, email válido, senha ≥ MinPasswordLen e sem espaços.
func ValidarCadastroMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		defer r.Body.Close()

		var req model.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// Nome
		req.Nome = strings.TrimSpace(req.Nome)
		if len(req.Nome) < 2 {
			http.Error(w, "Nome muito curto", http.StatusBadRequest)
			return
		}

		// E-mail
		normEmail, err := normalizeEmail(req.Email)
		if err != nil {
			// mensagens mais amigáveis (sem mudar status/mídia)
			switch {
			case err == http.ErrNoLocation:
				http.Error(w, "E-mail é obrigatório", http.StatusBadRequest)
			default:
				http.Error(w, "E-mail inválido", http.StatusBadRequest)
			}
			return
		}
		req.Email = normEmail

		// Senha
		if len(req.Senha) < model.MinPasswordLen {
			http.Error(w, "Senha muito curta (mínimo "+strconvI(model.MinPasswordLen)+" caracteres)", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços!", http.StatusBadRequest)
			return
		}

		// Reinsere JSON corrigido no corpo
		bodyBytes, _ := json.Marshal(req)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		next(w, r)
	}
}

// ValidarLoginMiddleware valida o payload de login.
// Regras: email válido e senha ≥ MinPasswordLen, sem espaços.
func ValidarLoginMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
		defer r.Body.Close()

		var req model.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// E-mail
		normEmail, err := normalizeEmail(req.Email)
		if err != nil {
			switch {
			case err == http.ErrNoLocation:
				http.Error(w, "E-mail é obrigatório", http.StatusBadRequest)
			default:
				http.Error(w, "E-mail inválido", http.StatusBadRequest)
			}
			return
		}
		req.Email = normEmail

		// Senha
		if len(req.Senha) < model.MinPasswordLen {
			http.Error(w, "Senha deve ter pelo menos "+strconvI(model.MinPasswordLen)+" caracteres.", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços!", http.StatusBadRequest)
			return
		}

		// Reinsere JSON corrigido no corpo
		bodyBytes, _ := json.Marshal(req)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		next(w, r)
	}
}

// ValidarEstudanteEmailMiddleware valida somente o campo "email" do estudante,
// preservando o JSON original (campos extras são mantidos).
func ValidarEstudanteEmailMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		orig, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil {
			http.Error(w, "Falha ao ler corpo da requisição", http.StatusBadRequest)
			return
		}

		// Preserva o payload como map genérico
		var payload map[string]any
		if err := json.Unmarshal(orig, &payload); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		rawEmail, _ := payload["email"].(string)
		normEmail, err := normalizeEmail(rawEmail)
		if err != nil {
			switch {
			case err == http.ErrNoLocation:
				http.Error(w, "E-mail do estudante é obrigatório", http.StatusBadRequest)
			default:
				http.Error(w, "E-mail do estudante inválido", http.StatusBadRequest)
			}
			return
		}

		// Atualiza somente o campo email e segue
		payload["email"] = normEmail
		normBody, _ := json.Marshal(payload)
		r.Body = io.NopCloser(bytes.NewReader(normBody))

		next(w, r)
	}
}

// strconvI converte int para string sem importar strconv inteiro
func strconvI(n int) string {
	// pequena função inline para evitar importar strconv só por isso
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}
