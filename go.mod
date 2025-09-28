// ===========================================
// 📦 go.mod — Configuração do Módulo Go
// -------------------------------------------
// Este arquivo define:
//  1) O nome do módulo raiz (backend).
//  2) A versão mínima do Go necessária.
//  3) Dependências externas (direct e indirect).
//
// ⚠️ Observações:
// - Nunca edite `go.sum` manualmente, ele é
//   gerado automaticamente pelo Go.
// - Para adicionar libs use: go get <pacote>
// - Para atualizar tudo:    go get -u ./...
// ===========================================

module backend // Nome do módulo. Usado nos imports: "backend/..."

// Versão mínima do Go usada no projeto
go 1.24.0

toolchain go1.24.5

require (
	// =============================
	// 📚 Dependências diretas
	// =============================

	// Driver PostgreSQL para Go
	github.com/lib/pq v1.10.9

	// Pacote oficial com utilitários de criptografia
	// (usado para hashing de senhas com bcrypt, etc.)
	golang.org/x/crypto v0.42.0
)

// =============================
// 📚 Dependências indiretas
// =============================
// - Instaladas automaticamente por outras libs
// - Podem ser atualizadas/removidas via `go mod tidy`
require github.com/joho/godotenv v1.5.1

require (
	cloud.google.com/go/auth v0.16.5 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.8.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/oauth2 v0.31.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/api v0.250.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250908214217-97024824d090 // indirect
	google.golang.org/grpc v1.75.1 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
)
