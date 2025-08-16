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
go 1.24

require (
    // =============================
    // 📚 Dependências diretas
    // =============================

    // Driver PostgreSQL para Go
    github.com/lib/pq v1.10.9

    // Pacote oficial com utilitários de criptografia
    // (usado para hashing de senhas com bcrypt, etc.)
    golang.org/x/crypto v0.40.0
)

// =============================
// 📚 Dependências indiretas
// =============================
// - Instaladas automaticamente por outras libs
// - Podem ser atualizadas/removidas via `go mod tidy`
require github.com/joho/godotenv v1.5.1 
