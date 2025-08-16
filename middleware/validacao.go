// backend/middleware/validacao.go
//
// 🔹 Objetivo deste arquivo:
//
//	Reunir middlewares de **validação de entrada** para os fluxos de cadastro, login
//	e criação/edição de estudantes. Garantem que os dados recebidos pela API
//	estejam corretos, normalizados e consistentes, antes de serem processados
//	pelos handlers principais.
//
// ===============================================================
// 📌 Estruturas de Request
// ===============================================================
//
// RegisterRequest → usado no fluxo de cadastro de usuários
// LoginRequest    → usado no fluxo de login de usuários
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Estrutura usada no cadastro de usuário
type RegisterRequest struct {
	Nome  string `json:"nome"`
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// Estrutura usada no login de usuário
type LoginRequest struct {
	Email string `json:"email"`
	Senha string `json:"senha"`
}

// ===============================================================
// 📌 Middleware: Validação de Cadastro
// ===============================================================
//
// Regras aplicadas:
//   - Nome → não pode ser vazio, mínimo de 2 caracteres
//   - E-mail → sem espaços, sem espaços nas bordas, formato válido
//   - Senha → mínimo 8 caracteres, sem espaços
//
// Após validação, substitui o corpo da requisição pelo JSON corrigido.
// Assim, o handler seguinte recebe os dados normalizados.
func ValidarCadastroMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// === Nome ===
		req.Nome = strings.TrimSpace(req.Nome)
		if len(req.Nome) < 2 {
			http.Error(w, "Nome muito curto", http.StatusBadRequest)
			return
		}

		// === E-mail ===
		if req.Email == "" || req.Email != strings.TrimSpace(req.Email) {
			http.Error(w, "E-mail não pode começar ou terminar com espaço!", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail não pode conter espaços!", http.StatusBadRequest)
			return
		}
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(req.Email) {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}

		// === Senha ===
		if req.Senha == "" || len(req.Senha) < 8 {
			http.Error(w, "Senha muito curta (mínimo 8 caracteres)", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços!", http.StatusBadRequest)
			return
		}

		// Reinsere JSON corrigido no corpo
		bodyBytes, _ := json.Marshal(req)
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		next(w, r)
	}
}

// ===============================================================
// 📌 Middleware: Validação de Login
// ===============================================================
//
// Regras aplicadas:
//   - E-mail → não pode conter espaços, nem bordas com espaços, formato válido
//   - Senha  → mínimo de 8 caracteres, sem espaços
//
// Normaliza o JSON e reenvia para o handler.
func ValidarLoginMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// === E-mail ===
		if req.Email == "" || req.Email != strings.TrimSpace(req.Email) {
			http.Error(w, "E-mail não pode começar ou terminar com espaço!", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Email, " ") {
			http.Error(w, "E-mail não pode conter espaços!", http.StatusBadRequest)
			return
		}
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(req.Email) {
			http.Error(w, "E-mail inválido", http.StatusBadRequest)
			return
		}

		// === Senha ===
		if req.Senha == "" || len(req.Senha) < 8 {
			http.Error(w, "Senha deve ter pelo menos 8 caracteres.", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Senha, " ") {
			http.Error(w, "Senha não pode conter espaços!", http.StatusBadRequest)
			return
		}

		// Reinsere JSON corrigido no corpo
		bodyBytes, _ := json.Marshal(req)
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		next(w, r)
	}
}

// ===============================================================
// 📌 Middleware: Validação de E-mail do Estudante
// ===============================================================
//
// 🔹 Objetivo:
//   - Valida apenas o campo "email" do estudante
//   - Não interfere em outros campos (nome, cpf, data_nascimento, etc.)
//   - Preserva o JSON completo enviado pelo frontend
//
// 🔹 Regras aplicadas:
//   - Campo "email" obrigatório
//   - Não pode conter espaços
//   - Deve estar em formato válido
//
// 🔹 Fluxo:
//  1. Lê corpo original
//  2. Decodifica em `map[string]any` (preserva campos extras)
//  3. Valida e normaliza e-mail
//  4. Reconstrói JSON e envia para o próximo handler
func ValidarEstudanteEmailMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1) Lê corpo original
		orig, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Falha ao ler corpo da requisição", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// 2) Deserializa para map genérico
		var payload map[string]any
		if err := json.Unmarshal(orig, &payload); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		// 3) Extrai e valida campo "email"
		rawEmail, _ := payload["email"].(string)
		email := strings.TrimSpace(rawEmail)

		if email == "" {
			http.Error(w, "E-mail do estudante é obrigatório", http.StatusBadRequest)
			return
		}
		if strings.Contains(email, " ") {
			http.Error(w, "E-mail do estudante não pode conter espaços", http.StatusBadRequest)
			return
		}
		emailRe := regexp.MustCompile(`^[\w\-.]+@([\w-]+\.)+[\w-]{2,4}$`)
		if !emailRe.MatchString(email) {
			http.Error(w, "E-mail do estudante inválido", http.StatusBadRequest)
			return
		}

		// 4) Normaliza e reinsere
		payload["email"] = email
		normBody, _ := json.Marshal(payload)

		// 5) Reatribui corpo e segue para o handler
		r.Body = io.NopCloser(bytes.NewReader(normBody))
		next(w, r)
	}
}
