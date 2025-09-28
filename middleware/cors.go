/*
/// Projeto: Tecmise
/// Arquivo: backend/middleware/cors.go
/// Responsabilidade: Middleware CORS configurável por variáveis de ambiente (origens, métodos, cabeçalhos, credenciais, max-age).
/// Dependências principais: net/http, os, strings.
/// Pontos de atenção:
/// - Este middleware pode coexistir com o CORS inline definido em main.go; evite duplicidade ao aplicar ambos.
/// - Quando CORS_ALLOW_CREDENTIALS=true, Access-Control-Allow-Origin nunca será "*" (espelha a Origin permitida).
/// - Cabeçalhos expostos (Access-Control-Expose-Headers) não são definidos; adicionar se o frontend precisar ler headers custom.
/// - Header "Vary: Origin" é adicionado; útil para caches, mas duplicações podem ocorrer se outro CORS também adicioná-lo.
*/

//
// backend/middleware/cors.go
//
// Middleware CORS configurável por ambiente.
// Compatível com o uso atual do projeto e alinhado ao comportamento do main.go.
//
// Variáveis de ambiente (opcionais):
// - CORS_ALLOW_ORIGINS   → "*" (default) ou lista separada por vírgula
// - CORS_ALLOW_METHODS   → "GET, POST, PUT, DELETE, OPTIONS" (default)
// - CORS_ALLOW_HEADERS   → "Content-Type, X-User-Email" (default)
// - CORS_MAX_AGE         → "86400" (segundos, default 24h)
// - CORS_ALLOW_CREDENTIALS → "true" para enviar Access-Control-Allow-Credentials: true
//
// Observação: se CORS_ALLOW_CREDENTIALS=true, o cabeçalho Access-Control-Allow-Origin
// nunca será "*" — refletimos a Origin da requisição quando permitida.

package middleware

import (
	"net/http"
	"os"
	"strings"
)

/// ============ Configurações & Constantes ============

/**
 * getEnv retorna o valor da variável de ambiente (trim) ou um default se vazia/ausente.
 * Uso: leitura de configurações CORS (origens, métodos, cabeçalhos, etc.).
 */
func getEnv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

/**
 * splitCSV divide uma string por vírgulas em itens não vazios já "trimados".
 * Ex.: "https://a.com, https://b.com" -> []string{"https://a.com","https://b.com"}
 */
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

/**
 * originAllowed verifica se uma origem é aceita pela lista configurada.
 * Regras:
 * - Lista vazia -> false
 * - Primeiro item "*" -> qualquer origem permitida
 * - Caso contrário, compara igualdade literal com a Origin recebida
 */
func originAllowed(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	if allowed[0] == "*" {
		return true
	}
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	return false
}

/// ============ Funções Públicas (Middlewares) ============

/**
 * Cors adiciona cabeçalhos CORS e trata requisições de pré-flight (OPTIONS).
 *
 * Variáveis de ambiente suportadas:
 * - CORS_ALLOW_ORIGINS (CSV ou "*")
 * - CORS_ALLOW_METHODS (default: "GET, POST, PUT, DELETE, OPTIONS")
 * - CORS_ALLOW_HEADERS (default: "Content-Type, X-User-Email")
 * - CORS_MAX_AGE (segundos como string, default: "86400")
 * - CORS_ALLOW_CREDENTIALS ("true" para habilitar credenciais)
 *
 * Comportamento:
 * - Sempre adiciona "Vary: Origin".
 * - Se credenciais habilitadas, espelha a Origin permitida e define "Access-Control-Allow-Credentials: true".
 * - Caso contrário, usa "*" se habilitado globalmente, ou espelha Origins específicas.
 * - Responde 200 em OPTIONS com cabeçalhos CORS configurados.
 */
func Cors(next http.Handler) http.Handler {
	allowedOrigins := splitCSV(getEnv("CORS_ALLOW_ORIGINS", "*"))
	allowedMethods := getEnv("CORS_ALLOW_METHODS", "GET, POST, PUT, DELETE, OPTIONS")
	allowedHeaders := getEnv("CORS_ALLOW_HEADERS", "Content-Type, X-User-Email")
	maxAge := getEnv("CORS_MAX_AGE", "86400")
	allowCreds := strings.EqualFold(getEnv("CORS_ALLOW_CREDENTIALS", "false"), "true")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Sempre variar por Origin para caches corretos
		w.Header().Add("Vary", "Origin")

		// Definição de origem permitida
		if allowCreds {
			// Com credenciais não podemos usar "*"
			if origin != "" && originAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		} else {
			// Modo aberto por padrão
			if len(allowedOrigins) > 0 && allowedOrigins[0] == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" && originAllowed(origin, allowedOrigins) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}

		// Métodos e cabeçalhos
		w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
		w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		w.Header().Set("Access-Control-Max-Age", maxAge)

		// Pré-flight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
