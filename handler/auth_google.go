/*
/// Projeto: Tecmise
/// Arquivo: backend/handler/auth_google.go
/// Responsabilidade: Endpoint de autenticação via Google Identity Services (GIS) utilizando validação de ID Token e upsert de usuário via repositório do pacote model.
/// Dependências principais: google.golang.org/api/idtoken, backend/model (UserRepository), net/http.
/// Pontos de atenção:
/// - Requer a variável de ambiente GOOGLE_CLIENT_ID para validar o "aud" do token.
/// - Não verifica "email_verified" nas claims; considerar se necessário.
/// - Erros retornados são genéricos por design (sem detalhes sensíveis); logs podem ser adicionados em camadas superiores.
/// - Tamanho do body limitado a 1 MiB. Content-Type esperado: application/json.
/// - Reutiliza helpers writeJSON / writeJSONError (definidos no package) – este arquivo pressupõe sua existência no mesmo pacote.
*/

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/model"

	"google.golang.org/api/idtoken"
)

// 🔐 Login com Google (GIS) — usa o repositório do package model.
// ⚠️ Não declaramos helpers writeJSON/writeJSONError aqui; reutilizamos os do package.

/// ============ Tipos & Estruturas ============

/**
 * AuthGoogleHandler encapsula dependências para o fluxo de login com Google.
 * Campos:
 *  - repo: implementação de model.UserRepository responsável por upsert de usuários.
 *  - clientID: Client ID OAuth do Google (usado na validação do ID Token).
 *  - timeout: tempo máximo para validar token e executar operações (context deadline).
 */
type AuthGoogleHandler struct {
	repo     model.UserRepository
	clientID string
	timeout  time.Duration
}

/**
 * NewAuthGoogleHandler cria uma instância do handler usando GOOGLE_CLIENT_ID de os.Getenv.
 * Observação: o valor é capturado na construção; alterações futuras na env não afetarão instâncias existentes.
 * Exemplo:
 *   h := handler.NewAuthGoogleHandler(model.NewUserRepo(db))
 */
func NewAuthGoogleHandler(repo model.UserRepository) *AuthGoogleHandler {
	return &AuthGoogleHandler{
		repo:     repo,
		clientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		timeout:  8 * time.Second,
	}
}

/**
 * RegisterRoutes registra a rota POST /login/google na mux fornecida.
 * Nota: no main.go, a rota é registrada manualmente; este método é opcional/conveniente.
 */
func (h *AuthGoogleHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/login/google", h.LoginGoogle)
}

// ===== DTOs =====

/**
 * googleLoginRequest representa o corpo aceito pelo endpoint,
 * contemplando variações comuns que o GIS pode fornecer.
 */
type googleLoginRequest struct {
	// Aceita as variações comuns do GIS
	IDToken    string `json:"idToken"`
	IDTokenAlt string `json:"id_token"`
	Credential string `json:"credential"`
}

/**
 * loginResponse é a resposta mínima esperada pelo frontend após autenticação com sucesso.
 */
type loginResponse struct {
	ID    int    `json:"id"`
	Nome  string `json:"nome"`
	Email string `json:"email"`
}

// ===== Handler =====

/**
 * LoginGoogle (POST /login/google)
 *
 * Fluxo:
 *  1) Valida método HTTP (aceita apenas POST).
 *  2) Garante presença de GOOGLE_CLIENT_ID.
 *  3) Lê e parseia JSON do corpo (limite 1 MiB).
 *  4) Extrai idToken de campos aceitos (idToken, id_token, credential).
 *  5) Valida o ID Token com audience = GOOGLE_CLIENT_ID (idtoken.Validate).
 *  6) Extrai claims relevantes (email, name, picture, sub).
 *  7) Upsert no repositório de usuários via model.UserRepository.
 *  8) Retorna 200 com {id, nome, email} em sucesso; erros com http.Status adequados.
 *
 * Efeitos colaterais:
 *  - Usa context.WithTimeout com h.timeout.
 *  - Não grava sessão/cookie; apenas responde JSON com os dados mínimos.
 */
func (h *AuthGoogleHandler) LoginGoogle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Método não permitido")
		return
	}
	if h.clientID == "" {
		writeJSONError(w, http.StatusInternalServerError, "Servidor sem GOOGLE_CLIENT_ID configurado")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Falha ao ler corpo")
		return
	}
	defer r.Body.Close()

	var req googleLoginRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON inválido")
		return
	}

	// Aceita idToken em múltiplos campos (idToken, id_token, credential)
	idToken := firstNonEmpty(req.IDToken, req.IDTokenAlt, req.Credential)
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		writeJSONError(w, http.StatusBadRequest, "idToken é obrigatório")
		return
	}

	// Valida o ID Token (audience = GOOGLE_CLIENT_ID)
	payload, err := idtoken.Validate(ctx, idToken, h.clientID)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "ID Token inválido para este CLIENT_ID")
		return
	}

	// Claims básicas
	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)
	sub, _ := payload.Claims["sub"].(string)

	if email == "" || sub == "" {
		writeJSONError(w, http.StatusUnauthorized, "Claims obrigatórias ausentes no token")
		return
	}
	if name == "" {
		name = email
	}

	// Upsert no repositório
	u, err := h.repo.UpsertFromGoogle(ctx, name, email, sub, picture)
	if err != nil || u == nil {
		writeJSONError(w, http.StatusInternalServerError, "Falha ao autenticar com Google")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		ID:    u.ID,
		Nome:  u.Nome,
		Email: u.Email,
	})
}

// ===== helpers =====

/**
 * firstNonEmpty retorna o primeiro valor não-vazio em uma lista de strings.
 * Útil para aceitar múltiplos aliases do token no payload.
 */
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
