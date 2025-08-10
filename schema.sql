CREATE TABLE IF NOT EXISTS usuarios (
    id serial PRIMARY KEY,
    nome VARCHAR(100), 
    email VARCHAR(200) NOT NULL UNIQUE,
    senha_hash VARCHAR(300) NOT NULL
);