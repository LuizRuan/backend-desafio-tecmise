/*
/// Projeto: Tecmise
/// Arquivo: backend/model/user_repo.go
/// Responsabilidade: Repositório de usuários (PostgreSQL) com fluxo de UPSERT para autenticação via Google (GIS).
/// Dependências principais: database/sql (Postgres), information_schema.columns, pacote local model.User.
/// Pontos de atenção:
/// - Concurrency: cache de schema (schemaChecked/hasGoogleSub/hasFotoURL) não é protegido por mutex; possível data race se usado por múltiplas goroutines.
/// - Idempotência/Concorrência: upsert não usa transação; disputas podem criar duplicatas se o banco não tiver UNIQUE(email)/UNIQUE(google_sub).
/// - Schema discovery: verificação usa information_schema por nome de tabela sem schema qualificado; depende de search_path (padrão "public").
/// - Case-insensitive por LOWER(email) pode impactar uso de índices; CITEXT seria mais eficiente.
/// - Atualizações (google_sub/foto_url) são separadas e sem transação; em falha parcial pode haver estado intermediário.
*/

package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// -----------------------------------------------------------------------------
// UserRepository para login Google (tabela: usuarios)
// -----------------------------------------------------------------------------
//
// Observação importante sobre senha_hash:
// Sua tabela `usuarios` exige `senha_hash` NOT NULL. Como contas Google não
// usam senha local, gravamos `senha_hash` como string vazia (`''`) apenas para
// satisfazer a restrição. Isso impede login por e-mail/senha para esses
// usuários (bcrypt vai falhar), o que é desejado nesse fluxo.
//
// Tabela mínima esperada:
//   usuarios(id, nome, email, senha_hash [, google_sub] [, foto_url])
//

/// ============ Tipos & Interfaces ============

// UserRepository define o contrato de persistência para o fluxo de autenticação Google.
type UserRepository interface {
	// UpsertFromGoogle:
	// 1) Se existir usuarios.google_sub = sub -> retorna usuário.
	// 2) Senão, se existir usuarios.email = email -> (se possível) vincula google_sub e retorna.
	// 3) Senão, cria usuário (com google_sub/foto_url se colunas existirem).
	UpsertFromGoogle(ctx context.Context, nome, email, sub, picture string) (*User, error)
}

// SQLUserRepo implementação baseada em database/sql para PostgreSQL.
// Mantém um cache simples de detecção de colunas opcionais (google_sub, foto_url).
type SQLUserRepo struct {
	db *sql.DB

	// Descoberta de schema (cache simples)
	schemaChecked bool
	hasGoogleSub  bool
	hasFotoURL    bool
}

/// ============ Inicialização/Bootstrap ============

// NewUserRepo cria uma instância de SQLUserRepo com o pool *sql.DB informado.
// Exemplo de uso:
//
//	repo := model.NewUserRepo(db)
//	user, err := repo.UpsertFromGoogle(ctx, "Nome", "email@dominio.com", sub, picture)
func NewUserRepo(db *sql.DB) *SQLUserRepo { return &SQLUserRepo{db: db} }

/// ============ Funções Internas (helpers) ============

// ensureSchema detecta (uma única vez) a existência das colunas opcionais na tabela `usuarios`.
// Observações:
// - A detecção depende do search_path do banco (table_name = 'usuarios').
// - Em erro na consulta, assume coluna ausente (retorna false).
func (r *SQLUserRepo) ensureSchema(ctx context.Context) {
	if r.schemaChecked {
		return
	}
	check := func(col string) bool {
		const q = `
			SELECT 1
			  FROM information_schema.columns
			 WHERE table_name = 'usuarios' AND column_name = $1
			 LIMIT 1`
		var x int
		err := r.db.QueryRowContext(ctx, q, strings.ToLower(col)).Scan(&x)
		return err == nil
	}
	r.hasGoogleSub = check("google_sub")
	r.hasFotoURL = check("foto_url")
	r.schemaChecked = true
}

/// ============ Funções Públicas ============

// UpsertFromGoogle realiza um "upsert" manual de usuário baseado nos dados do Google.
// Estratégia:
//  1. Se google_sub existir e corresponder, retorna.
//  2. Caso contrário, tenta por email (case-insensitive); se achar, vincula google_sub/foto_url (se colunas existirem).
//  3. Se não encontrar, insere novo usuário preenchendo senha_hash = ” para satisfazer NOT NULL.
//
// Erros: encapsulados via fmt.Errorf com contexto da operação.
func (r *SQLUserRepo) UpsertFromGoogle(ctx context.Context, nome, email, sub, picture string) (*User, error) {
	r.ensureSchema(ctx)

	// ---------- 1) busca por google_sub ----------
	if r.hasGoogleSub && sub != "" {
		const q = `SELECT id, nome, email, COALESCE(foto_url,'') FROM usuarios WHERE google_sub = $1`
		u := &User{}
		err := r.db.QueryRowContext(ctx, q, sub).Scan(&u.ID, &u.Nome, &u.Email, &u.FotoURL)
		if err == nil {
			return u, nil
		}
		// Se chegamos aqui, err é != nil (pois o caminho de err == nil já retornou).
		// Para evitar o aviso do linter (condição tautológica), testamos apenas o tipo do erro.
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("query por google_sub: %w", err)
		}
	}

	// ---------- 2) busca por email (case-insensitive) ----------
	{
		const qSel = `SELECT id, nome, email, COALESCE(foto_url,'') FROM usuarios WHERE LOWER(email) = LOWER($1)`
		u := &User{}
		err := r.db.QueryRowContext(ctx, qSel, email).Scan(&u.ID, &u.Nome, &u.Email, &u.FotoURL)
		if err == nil {
			// vincula sub se a coluna existir
			if r.hasGoogleSub && sub != "" {
				if _, err := r.db.ExecContext(ctx, `UPDATE usuarios SET google_sub = $1 WHERE id = $2`, sub, u.ID); err != nil {
					return nil, fmt.Errorf("vincular google_sub: %w", err)
				}
			}
			// atualiza foto se a coluna existir e vier valor novo
			if r.hasFotoURL && picture != "" && picture != u.FotoURL {
				if _, err := r.db.ExecContext(ctx, `UPDATE usuarios SET foto_url = $1 WHERE id = $2`, picture, u.ID); err != nil {
					return nil, fmt.Errorf("atualizar foto_url: %w", err)
				}
				u.FotoURL = picture
			}
			return u, nil
		}
		// Mesmo racional: se estamos aqui, err != nil; testamos somente o tipo.
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("query por email: %w", err)
		}
	}

	// ---------- 3) cria novo usuário ----------
	// IMPORTANTE: sempre preencher senha_hash = '' para satisfazer NOT NULL.
	switch {
	case r.hasGoogleSub && r.hasFotoURL:
		const qIns = `
			INSERT INTO usuarios (nome, email, senha_hash, google_sub, foto_url)
			VALUES ($1, $2, '', $3, $4)
			RETURNING id, nome, email, COALESCE(foto_url,'')`
		u := &User{}
		if err := r.db.QueryRowContext(ctx, qIns, nome, email, sub, picture).
			Scan(&u.ID, &u.Nome, &u.Email, &u.FotoURL); err != nil {
			return nil, fmt.Errorf("inserir (com google_sub/foto_url): %w", err)
		}
		return u, nil

	case r.hasGoogleSub && !r.hasFotoURL:
		const qIns = `
			INSERT INTO usuarios (nome, email, senha_hash, google_sub)
			VALUES ($1, $2, '', $3)
			RETURNING id, nome, email, ''`
		u := &User{}
		if err := r.db.QueryRowContext(ctx, qIns, nome, email, sub).
			Scan(&u.ID, &u.Nome, &u.Email, &u.FotoURL); err != nil {
			return nil, fmt.Errorf("inserir (com google_sub): %w", err)
		}
		return u, nil

	case !r.hasGoogleSub && r.hasFotoURL:
		const qIns = `
			INSERT INTO usuarios (nome, email, senha_hash, foto_url)
			VALUES ($1, $2, '', $3)
			RETURNING id, nome, email, COALESCE(foto_url,'')`
		u := &User{}
		if err := r.db.QueryRowContext(ctx, qIns, nome, email, picture).
			Scan(&u.ID, &u.Nome, &u.Email, &u.FotoURL); err != nil {
			return nil, fmt.Errorf("inserir (com foto_url): %w", err)
		}
		return u, nil
	}

	// Sem colunas extras -> insere somente nome/email/senha_hash
	const qIns = `
		INSERT INTO usuarios (nome, email, senha_hash)
		VALUES ($1, $2, '')
		RETURNING id, nome, email, ''`
	u := &User{}
	if err := r.db.QueryRowContext(ctx, qIns, nome, email).
		Scan(&u.ID, &u.Nome, &u.Email, &u.FotoURL); err != nil {
		return nil, fmt.Errorf("inserir (básico): %w", err)
	}
	return u, nil
}
