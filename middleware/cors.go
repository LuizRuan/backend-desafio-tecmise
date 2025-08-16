// backend/middleware/cors.go
//
// Middleware responsável por habilitar CORS (Cross-Origin Resource Sharing).
// Permite que clientes frontend hospedados em domínios diferentes do backend
// consigam consumir a API sem bloqueios do navegador.
//
// 🔹 Como funciona:
//   - Adiciona cabeçalhos HTTP em todas as respostas:
//   - Access-Control-Allow-Origin: "*"   → aceita requisições de qualquer origem
//   - Access-Control-Allow-Methods       → define métodos HTTP aceitos (GET, POST, PUT, DELETE, OPTIONS)
//   - Access-Control-Allow-Headers       → libera cabeçalhos personalizados usados no frontend
//   - Se a requisição for do tipo OPTIONS (pré-flight), retorna imediatamente 200 OK.
//   - Caso contrário, passa o fluxo para o próximo handler da cadeia.
//
// 🔹 Pontos de atenção:
//   - Em produção, o ideal é substituir "*" por domínios específicos confiáveis.
//   - O cabeçalho "X-User-Email" foi incluído para permitir autenticação/identificação
//     personalizada no frontend TecMise.
//
// 🔹 Uso típico no main.go:
//
//	mux := http.NewServeMux()
//	handler := middleware.Cors(mux)
//	log.Fatal(http.ListenAndServe(":8080", handler))
package middleware

import "net/http"

// Cors adiciona os cabeçalhos necessários para permitir requisições cross-origin
// e trata pré-flight requests (OPTIONS).
func Cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Define permissões de origem, métodos e cabeçalhos
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-Email")

		// Se for pré-flight, retorna OK sem chamar o próximo handler
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Encaminha requisição para o próximo handler
		next.ServeHTTP(w, r)
	})
}
