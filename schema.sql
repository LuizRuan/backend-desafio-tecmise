-- schema.sql
--
-- 📦 Estrutura inicial do banco de dados TecMise
--
-- Objetivo:
--   Este script cria as tabelas necessárias para autenticação de usuários.
--   O foco aqui é a tabela `usuarios`, que guarda credenciais e dados básicos.
--
-- Boas práticas seguidas:
-- - `IF NOT EXISTS`: evita erro ao rodar migrations múltiplas vezes.
-- - `id serial PRIMARY KEY`: chave primária incremental.
-- - `email UNIQUE`: garante que não existam contas duplicadas.
-- - `senha_hash`: senha nunca é armazenada em texto puro, sempre hash.
--
-- Próximos passos:
-- - Criar tabelas para `estudantes` e `anos` (com foreign key para `usuarios`).
-- - Adicionar índice em colunas de busca frequente (ex: email).
-- - Avaliar constraints de integridade referencial entre tabelas.

CREATE TABLE IF NOT EXISTS usuarios (
    id SERIAL PRIMARY KEY,           -- Identificador único (auto incremento)
    nome VARCHAR(100),               -- Nome do usuário (não obrigatório)
    email VARCHAR(200) NOT NULL UNIQUE, -- Email único, obrigatório (login)
    senha_hash VARCHAR(300) NOT NULL    -- Hash seguro da senha (bcrypt/argon2)
);
